package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func dnsCmd() *cobra.Command {
	var set []string
	var auto, yes bool
	c := &cobra.Command{
		Use:   "dns",
		Short: "Show or set the network DNS resolvers (--set to pin, --auto for ISP default)",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			if auto || len(set) > 0 {
				if !yes {
					return fmt.Errorf("changing DNS restarts resolution for every client: re-run with --yes")
				}
				mode := "custom"
				if auto {
					mode = "automatic"
				}
				if err := client.SetDNS(ctx, id, mode, set); err != nil {
					return err
				}
				fmt.Println("dns updated; allow ~30s to take effect")
			}
			n, err := client.Network(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(n.DNS)
			}
			ips := n.DNS.Parent.IPs
			if n.DNS.Custom != nil && len(n.DNS.Custom.IPs) > 0 {
				ips = n.DNS.Custom.IPs
			}
			table([][]string{
				{"mode", n.DNS.Mode},
				{"resolvers", strings.Join(ips, ", ")},
				{"caching", fmt.Sprint(n.DNS.Caching)},
			})
			if len(ips) > 1 && n.DNS.Mode == "custom" {
				fmt.Println("\nnote: clients may query any listed resolver, so a single private resolver enforces filtering.")
			}
			return nil
		},
	}
	c.Flags().StringSliceVar(&set, "set", nil, "custom resolver IPs, e.g. --set 10.0.4.70")
	c.Flags().BoolVar(&auto, "auto", false, "revert to the ISP/eero default resolvers")
	c.Flags().BoolVar(&yes, "yes", false, "confirm the change")
	return c
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check config, session and reachability",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			ok := func(b bool) string {
				if b {
					return "ok"
				}
				return "FAIL"
			}
			hasToken := cfg.Token != ""
			fmt.Printf("%-5s config       %s\n", ok(true), cfgPath())
			fmt.Printf("%-5s session      %s\n", ok(hasToken), sessionWhere())
			if !hasToken {
				fmt.Println("\nrun `eerox login <email-or-phone>`")
				return nil
			}
			a, err := client.Account(ctx)
			fmt.Printf("%-5s api          %s\n", ok(err == nil), accountLine(a, err))
			if err != nil {
				return nil
			}
			id, nerr := netID(ctx)
			fmt.Printf("%-5s network      %s\n", ok(nerr == nil), id)
			if nerr == nil {
				if n, e := client.Network(ctx, id); e == nil {
					fmt.Printf("%-5s internet     %s\n", ok(n.Health.Internet.Status == "connected"), n.Health.Internet.Status)
				}
			}
			return nil
		},
	}
}
