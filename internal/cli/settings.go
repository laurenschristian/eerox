package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// networkToggles maps a CLI flag name to the eero network JSON key.
var networkToggles = map[string]string{
	"wpa3":          "wpa3",
	"sqm":           "sqm",
	"band-steering": "band_steering",
	"upnp":          "upnp",
	"ipv6":          "ipv6_upstream",
	"thread":        "thread",
}

func setCmd() *cobra.Command {
	vals := map[string]*bool{}
	var yes bool
	c := &cobra.Command{
		Use:   "set",
		Short: "Toggle network features (--wpa3, --sqm, --band-steering, --upnp, --ipv6, --thread)",
		Long: "Turn network features on or off, e.g. `eerox set --wpa3 on --sqm on --yes`.\n" +
			"With no flags it prints the current state. WPA3 can disconnect legacy 2.4GHz gear;\n" +
			"SQM (smart queue management) trades a little peak throughput for lower latency under load.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			fields := map[string]any{}
			for flag, key := range networkToggles {
				if cmd.Flags().Changed(flag) {
					fields[key] = *vals[flag]
				}
			}
			if len(fields) > 0 {
				if !yes {
					return fmt.Errorf("changing network settings affects every client: re-run with --yes")
				}
				if err := client.UpdateNetwork(ctx, id, fields); err != nil {
					return err
				}
				fmt.Printf("updated %d setting(s)\n", len(fields))
			}
			n, err := client.Network(ctx, id)
			if err != nil {
				return err
			}
			state := map[string]bool{"wpa3": n.Wpa3, "sqm": n.SQM, "band_steering": n.BandSteering, "upnp": n.Upnp, "ipv6_upstream": n.IPv6Upstream}
			if flagJSON {
				return emit(state)
			}
			keys := make([]string, 0, len(state))
			for k := range state {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			rows := make([][]string, 0, len(keys))
			for _, k := range keys {
				rows = append(rows, []string{k, onOff(state[k])})
			}
			table(rows)
			return nil
		},
	}
	names := make([]string, 0, len(networkToggles))
	for f := range networkToggles {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		b := new(bool)
		vals[f] = b
		c.Flags().Var(&onOffValue{b}, f, "on|off")
	}
	c.Flags().BoolVar(&yes, "yes", false, "confirm the change")
	return c
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// onOffValue is a pflag.Value accepting on/off/true/false/1/0.
type onOffValue struct{ p *bool }

func (o *onOffValue) String() string { return onOff(o.p != nil && *o.p) }
func (o *onOffValue) Type() string   { return "on|off" }
func (o *onOffValue) Set(s string) error {
	switch strings.ToLower(s) {
	case "on", "true", "1", "yes", "enable", "enabled":
		*o.p = true
	case "off", "false", "0", "no", "disable", "disabled":
		*o.p = false
	default:
		return fmt.Errorf("want on or off, got %q", s)
	}
	return nil
}
