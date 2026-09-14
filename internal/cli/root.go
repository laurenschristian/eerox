// Package cli wires the cobra commands.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/laurenschristian/eerox/internal/config"
	"github.com/laurenschristian/eerox/internal/eero"
)

var (
	Version = "dev"

	flagNetwork string
	flagJSON    bool

	cfg    *config.Config
	client *eero.Client
)

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "eerox",
		Short:         "CLI and MCP server for eero networks",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			var err error
			if cfg, err = config.Load(); err != nil {
				return err
			}
			if flagNetwork != "" {
				cfg.Network = flagNetwork
			}
			if err := cfg.ResolveToken(); err != nil {
				return err
			}
			client = eero.New(os.Getenv("EERO_BASE"), cfg.Token)
			client.OnToken = cfg.SaveToken
			switch cmd.Name() {
			case "login", "logout", "help", "completion":
				return nil
			}
			if cfg.Token == "" {
				return eero.ErrNotLoggedIn
			}
			return nil
		},
	}
	root.PersistentFlags().StringVar(&flagNetwork, "network", "", "network id (env EERO_NETWORK, default from login)")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "print raw JSON")

	root.AddCommand(
		loginCmd(), logoutCmd(), accountCmd(), networksCmd(), networkCmd(), statusCmd(),
		devicesCmd(), deviceCmd(), eerosCmd(), rebootCmd(), profilesCmd(), profileCmd(),
		reservationsCmd(), forwardsCmd(), guestCmd(), speedtestCmd(), exportCmd(), rawCmd(), mcpCmd(),
	)
	return root
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

func emit(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func table(rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, r := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	_ = w.Flush()
}

func prompt(label string) string {
	fmt.Print(label)
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s)
}

// netID returns the configured network, discovering it from the account when unset.
func netID(ctx context.Context) (string, error) {
	if cfg.Network != "" {
		return cfg.Network, nil
	}
	a, err := client.Account(ctx)
	if err != nil {
		return "", err
	}
	if len(a.Networks.Data) == 0 {
		return "", errors.New("account has no networks")
	}
	cfg.Network = a.Networks.Data[0].ID()
	_ = config.Save(cfg)
	return cfg.Network, nil
}

func loginCmd() *cobra.Command {
	var noKeychain bool
	c := &cobra.Command{
		Use:   "login [email-or-phone]",
		Short: "Log in (a verification code is sent to your email or phone)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id := cfg.Login
			if len(args) == 1 {
				id = args[0]
			}
			if id == "" {
				id = prompt("email or phone: ")
			}
			if id == "" {
				return errors.New("login identifier required")
			}
			cfg.Login = id
			cfg.Keychain = !noKeychain && config.KeychainAvailable()
			if _, err := client.Login(ctx, id); err != nil {
				return err
			}
			code := prompt("verification code: ")
			if err := client.Verify(ctx, code); err != nil {
				return err
			}
			cfg.Network = ""
			n, err := netID(ctx)
			if err != nil {
				return err
			}
			where := config.Path()
			if cfg.Keychain {
				where = "macOS Keychain (service eerox)"
			}
			fmt.Printf("logged in as %s, network %s, token in %s\n", id, n, where)
			return nil
		},
	}
	c.Flags().BoolVar(&noKeychain, "no-keychain", false, "store the token in the config file instead of the macOS Keychain")
	return c
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use: "logout", Short: "Forget the session token",
		RunE: func(_ *cobra.Command, _ []string) error {
			if cfg.Token != "" {
				ctx, cancel := ctx()
				defer cancel()
				_ = client.Logout(ctx)
			}
			return cfg.ClearToken()
		},
	}
}

func accountCmd() *cobra.Command {
	return &cobra.Command{
		Use: "account", Short: "Account owner, contact, premium status",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			a, err := client.Account(ctx)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(a)
			}
			table([][]string{
				{"name", a.Name}, {"email", a.Email.Value}, {"phone", a.Phone.Value},
				{"premium", a.PremiumStatus}, {"networks", fmt.Sprint(a.Networks.Count)},
			})
			return nil
		},
	}
}

func networksCmd() *cobra.Command {
	return &cobra.Command{
		Use: "networks", Short: "Networks on the account",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			a, err := client.Account(ctx)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(a.Networks.Data)
			}
			rows := [][]string{{"ID", "NAME", "CREATED"}}
			for _, n := range a.Networks.Data {
				rows = append(rows, []string{n.ID(), n.Name, n.Created})
			}
			table(rows)
			return nil
		},
	}
}

func networkCmd() *cobra.Command {
	return &cobra.Command{
		Use: "network", Short: "Network details: WAN, ISP, DHCP, DNS, health, last speed test",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			n, err := client.Network(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(n)
			}
			dns := n.DNS.Mode
			if n.DNS.Custom != nil && len(n.DNS.Custom.IPs) > 0 {
				dns += " " + strings.Join(n.DNS.Custom.IPs, ",")
			} else if len(n.DNS.Parent.IPs) > 0 {
				dns += " " + strings.Join(n.DNS.Parent.IPs, ",")
			}
			dhcp := n.DHCP.Mode
			if n.DHCP.Custom != nil {
				dhcp += fmt.Sprintf(" %s/%s %s-%s", n.DHCP.Custom.SubnetIP, n.DHCP.Custom.SubnetMask, n.DHCP.Custom.StartIP, n.DHCP.Custom.EndIP)
			}
			table([][]string{
				{"name", n.Name}, {"id", n.ID()}, {"status", n.Status},
				{"wan ip", n.WanIP}, {"isp", n.GeoIP.ISP + " (" + n.GeoIP.City + ")"},
				{"mode", n.Connection.Mode}, {"dhcp", dhcp}, {"dns", dns},
				{"internet", fmt.Sprintf("%s (isp up: %v)", n.Health.Internet.Status, n.Health.Internet.ISPUp)},
				{"mesh", n.Health.EeroNetwork.Status}, {"eeros", fmt.Sprint(n.Eeros.Count)},
				{"speed", fmt.Sprintf("%.0f down / %.0f up %s (%s)", n.Speed.Down.Value, n.Speed.Up.Value, n.Speed.Down.Units, n.Speed.Date)},
				{"firmware", fmt.Sprintf("%s (update: %v)", n.Updates.TargetFirmware, n.Updates.HasUpdate)},
				{"guest wifi", fmt.Sprintf("%v (%s)", n.GuestNetwork.Enabled, n.GuestNetwork.Name)},
				{"sqm", fmt.Sprint(n.SQM)}, {"upnp", fmt.Sprint(n.Upnp)}, {"wpa3", fmt.Sprint(n.Wpa3)},
				{"last reboot", n.LastReboot},
			})
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "One-line health: internet, mesh, nodes, clients",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			n, err := client.Network(ctx, id)
			if err != nil {
				return err
			}
			devs, err := client.Devices(ctx, id)
			if err != nil {
				return err
			}
			online := 0
			for _, d := range devs {
				if d.Connected {
					online++
				}
			}
			out := map[string]any{
				"network": n.Name, "internet": n.Health.Internet.Status, "mesh": n.Health.EeroNetwork.Status,
				"wan_ip": n.WanIP, "eeros": n.Eeros.Count, "devices_online": online, "devices_total": len(devs),
			}
			if flagJSON {
				return emit(out)
			}
			fmt.Printf("%s: internet %s, mesh %s, %d eeros, %d/%d devices online, wan %s\n",
				n.Name, n.Health.Internet.Status, n.Health.EeroNetwork.Status, n.Eeros.Count, online, len(devs), n.WanIP)
			return nil
		},
	}
}

type deviceFilter struct {
	online, offline, wired, wireless, guest, paused bool
	profile, search                                 string
}

func (f deviceFilter) keep(d eero.Device) bool {
	switch {
	case f.online && !d.Connected, f.offline && d.Connected,
		f.wired && d.Wireless, f.wireless && !d.Wireless,
		f.guest && !d.IsGuest, f.paused && !d.Paused:
		return false
	}
	if f.profile != "" && !strings.EqualFold(d.ProfileName(), f.profile) {
		return false
	}
	if f.search != "" {
		s := strings.ToLower(f.search)
		hay := strings.ToLower(strings.Join([]string{d.Name(), d.IP, d.MAC, d.Manufacturer, d.Hostname}, " "))
		if !strings.Contains(hay, s) {
			return false
		}
	}
	return true
}

func sortDevices(devs []eero.Device) {
	sort.SliceStable(devs, func(i, j int) bool {
		if devs[i].Connected != devs[j].Connected {
			return devs[i].Connected
		}
		return strings.ToLower(devs[i].Name()) < strings.ToLower(devs[j].Name())
	})
}

func devicesCmd() *cobra.Command {
	var f deviceFilter
	c := &cobra.Command{
		Use: "devices [search]", Short: "Client devices: name, ip, mac, node, status", Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 1 {
				f.search = args[0]
			}
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			devs, err := client.Devices(ctx, id)
			if err != nil {
				return err
			}
			var out []eero.Device
			for _, d := range devs {
				if f.keep(d) {
					out = append(out, d)
				}
			}
			sortDevices(out)
			if flagJSON {
				return emit(out)
			}
			rows := [][]string{{"ID", "NAME", "IP", "MAC", "STATE", "LINK", "NODE", "PROFILE", "LAST SEEN"}}
			for _, d := range out {
				rows = append(rows, []string{d.ID(), d.Name(), d.IP, d.MAC, deviceState(d), link(d), d.Source.Location, d.ProfileName(), ago(d.LastActive)})
			}
			table(rows)
			fmt.Printf("%d devices\n", len(out))
			return nil
		},
	}
	c.Flags().BoolVar(&f.online, "online", false, "connected only")
	c.Flags().BoolVar(&f.offline, "offline", false, "disconnected only")
	c.Flags().BoolVar(&f.wired, "wired", false, "wired only")
	c.Flags().BoolVar(&f.wireless, "wireless", false, "wireless only")
	c.Flags().BoolVar(&f.guest, "guest", false, "guest network only")
	c.Flags().BoolVar(&f.paused, "paused", false, "paused only")
	c.Flags().StringVar(&f.profile, "profile", "", "profile name")
	return c
}

func deviceState(d eero.Device) string {
	switch {
	case d.Blacklisted:
		return "blocked"
	case d.Paused:
		return "paused"
	case d.Connected:
		return "online"
	}
	return "offline"
}

func link(d eero.Device) string {
	if !d.Wireless {
		return "wired"
	}
	s := d.Interface.Frequency + d.Interface.FrequencyUnit
	if d.Connectivity.ScoreBars > 0 {
		s += fmt.Sprintf(" %d/5", d.Connectivity.ScoreBars)
	}
	return strings.TrimSpace(s)
}

func ago(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	d := time.Since(t).Round(time.Minute)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func findDevice(ctx context.Context, q string) (string, *eero.Device, error) {
	return findDeviceWith(ctx, client, netID, q)
}

func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

func deviceCmd() *cobra.Command {
	c := &cobra.Command{
		Use: "device <id|name|ip|mac>", Short: "Inspect one device", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, d, err := findDevice(ctx, args[0])
			if err != nil {
				return err
			}
			full, err := client.Device(ctx, id, d.ID())
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(full)
			}
			rows := [][]string{
				{"id", full.ID()}, {"name", full.Name()}, {"nickname", full.Nickname}, {"hostname", full.Hostname},
				{"ip", full.IP}, {"mac", full.MAC}, {"maker", full.Manufacturer}, {"model", full.ModelName}, {"type", full.DeviceType},
				{"state", deviceState(*full)}, {"link", link(*full)}, {"node", full.Source.Location},
				{"signal", fmt.Sprintf("%s (score %.2f, %s)", full.Connectivity.Signal, full.Connectivity.Score, full.Connectivity.RxBitrate)},
				{"profile", full.ProfileName()}, {"guest", fmt.Sprint(full.IsGuest)}, {"private", fmt.Sprint(full.IsPrivate)},
				{"first seen", full.FirstActive}, {"last seen", full.LastActive},
			}
			if full.Usage != nil {
				rows = append(rows, []string{"usage", fmt.Sprintf("%.1f down / %.1f up %s", full.Usage.Download, full.Usage.Upload, full.Usage.Units)})
			}
			table(rows)
			return nil
		},
	}
	c.AddCommand(
		deviceSet("rename <device> <name>", "Set the nickname", 2, func(a []string) map[string]any { return map[string]any{"nickname": a[1]} }),
		deviceSet("pause <device>", "Pause internet for the device", 1, func([]string) map[string]any { return map[string]any{"paused": true} }),
		deviceSet("unpause <device>", "Resume internet for the device", 1, func([]string) map[string]any { return map[string]any{"paused": false} }),
		deviceSet("block <device>", "Block the device from the network", 1, func([]string) map[string]any { return map[string]any{"blacklisted": true} }),
		deviceSet("unblock <device>", "Unblock the device", 1, func([]string) map[string]any { return map[string]any{"blacklisted": false} }),
	)
	return c
}

func deviceSet(use, short string, n int, fields func([]string) map[string]any) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Args: cobra.ExactArgs(n),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, d, err := findDevice(ctx, args[0])
			if err != nil {
				return err
			}
			if err := client.UpdateDevice(ctx, id, d.ID(), fields(args)); err != nil {
				return err
			}
			fmt.Printf("ok: %s (%s)\n", d.Name(), d.ID())
			return nil
		},
	}
}

func eerosCmd() *cobra.Command {
	return &cobra.Command{
		Use: "eeros", Short: "Mesh nodes: location, model, ip, status, mesh quality, clients",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			es, err := client.Eeros(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(es)
			}
			rows := [][]string{{"ID", "LOCATION", "MODEL", "IP", "STATUS", "ROLE", "LINK", "MESH", "CLIENTS", "OS", "UPDATE"}}
			for _, e := range es {
				role := "extender"
				if e.Gateway {
					role = "gateway"
				}
				l := "wireless"
				if e.Wired {
					l = "wired"
				}
				rows = append(rows, []string{e.ID(), e.Location, e.Model, e.IPAddress, e.Status, role, l, fmt.Sprintf("%d/5", e.MeshQualityBars), fmt.Sprint(e.ConnectedClientsCount), e.OSVersion, fmt.Sprint(e.UpdateAvailable)})
			}
			table(rows)
			return nil
		},
	}
}

func rebootCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use: "reboot [eero-id|location]", Short: "Reboot one eero, or the whole network with no argument", Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return errors.New("refusing without --yes")
			}
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return client.RebootNetwork(ctx, id)
			}
			es, err := client.Eeros(ctx, id)
			if err != nil {
				return err
			}
			for _, e := range es {
				if e.ID() == args[0] || strings.EqualFold(e.Location, args[0]) {
					fmt.Println("rebooting", e.Location)
					return client.RebootEero(ctx, e.ID())
				}
			}
			return fmt.Errorf("no eero matches %q", args[0])
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "confirm")
	return c
}

func profilesCmd() *cobra.Command {
	return &cobra.Command{
		Use: "profiles", Short: "Family profiles and their devices",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			ps, err := client.Profiles(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(ps)
			}
			rows := [][]string{{"ID", "NAME", "PAUSED", "DEVICES"}}
			for _, p := range ps {
				rows = append(rows, []string{p.ID(), p.Name, fmt.Sprint(p.Paused), fmt.Sprint(len(p.Devices))})
			}
			table(rows)
			return nil
		},
	}
}

func profileCmd() *cobra.Command {
	c := &cobra.Command{Use: "profile", Short: "Pause or resume a profile"}
	for _, x := range []struct {
		name string
		val  bool
	}{{"pause", true}, {"unpause", false}} {

		c.AddCommand(&cobra.Command{
			Use: x.name + " <profile-name|id>", Args: cobra.ExactArgs(1), Short: x.name + " every device in the profile",
			RunE: func(_ *cobra.Command, args []string) error {
				ctx, cancel := ctx()
				defer cancel()
				id, err := netID(ctx)
				if err != nil {
					return err
				}
				ps, err := client.Profiles(ctx, id)
				if err != nil {
					return err
				}
				for _, p := range ps {
					if p.ID() == args[0] || strings.EqualFold(p.Name, args[0]) {
						return client.UpdateProfile(ctx, id, p.ID(), map[string]any{"paused": x.val})
					}
				}
				return fmt.Errorf("no profile matches %q", args[0])
			},
		})
	}
	return c
}

func reservationsCmd() *cobra.Command {
	c := &cobra.Command{
		Use: "reservations", Short: "DHCP reservations", Aliases: []string{"dhcp"},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			rs, err := client.Reservations(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(rs)
			}
			rows := [][]string{{"ID", "IP", "MAC", "DESCRIPTION"}}
			for _, r := range rs {
				rows = append(rows, []string{r.ID(), r.IP, r.MAC, r.Description})
			}
			table(rows)
			return nil
		},
	}
	c.AddCommand(&cobra.Command{
		Use: "add <ip> <mac> [description]", Short: "Reserve an IP for a MAC", Args: cobra.RangeArgs(2, 3),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			desc := ""
			if len(args) == 3 {
				desc = args[2]
			}
			r, err := client.AddReservation(ctx, id, args[0], args[1], desc)
			if err != nil {
				return err
			}
			fmt.Println("reserved", r.IP, "for", r.MAC, "id", r.ID())
			return nil
		},
	}, &cobra.Command{
		Use: "rm <id|ip|mac>", Short: "Delete a reservation", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			rs, err := client.Reservations(ctx, id)
			if err != nil {
				return err
			}
			for _, r := range rs {
				if r.ID() == args[0] || r.IP == args[0] || strings.EqualFold(r.MAC, args[0]) {
					return client.DeleteReservation(ctx, id, r.ID())
				}
			}
			return fmt.Errorf("no reservation matches %q", args[0])
		},
	})
	return c
}

func forwardsCmd() *cobra.Command {
	return &cobra.Command{
		Use: "forwards", Short: "Port forwards",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			fs, err := client.Forwards(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(fs)
			}
			rows := [][]string{{"ID", "IP", "PROTO", "WAN PORT", "LAN PORT", "ENABLED", "DESCRIPTION"}}
			for _, f := range fs {
				rows = append(rows, []string{f.ID(), f.IP, f.Protocol, fmt.Sprint(f.GatewayPort), fmt.Sprint(f.ClientPort), fmt.Sprint(f.Enabled), f.Description})
			}
			table(rows)
			return nil
		},
	}
}

func guestCmd() *cobra.Command {
	var on, off bool
	var pass, name string
	c := &cobra.Command{
		Use: "guest", Short: "Guest wifi: show, --on/--off, --password, --name",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			fields := map[string]any{}
			if on {
				fields["enabled"] = true
			}
			if off {
				fields["enabled"] = false
			}
			if pass != "" {
				fields["password"] = pass
			}
			if name != "" {
				fields["name"] = name
			}
			if len(fields) > 0 {
				if err := client.UpdateGuestNetwork(ctx, id, fields); err != nil {
					return err
				}
			}
			g, err := client.GuestNetwork(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(g)
			}
			table([][]string{{"name", g.Name}, {"enabled", fmt.Sprint(g.Enabled)}, {"password", g.Password}})
			return nil
		},
	}
	c.Flags().BoolVar(&on, "on", false, "enable")
	c.Flags().BoolVar(&off, "off", false, "disable")
	c.Flags().StringVar(&pass, "password", "", "set password")
	c.Flags().StringVar(&name, "name", "", "set SSID")
	return c
}

func speedtestCmd() *cobra.Command {
	return &cobra.Command{
		Use: "speedtest", Short: "Run a speed test from the gateway (takes about 30s)",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			s, err := client.SpeedTest(ctx, id)
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(s)
			}
			fmt.Printf("down %.1f %s, up %.1f %s\n", s.Down.Value, s.Down.Units, s.Up.Value, s.Up.Units)
			return nil
		},
	}
}

// ExportEntry is the shape consumed by `adgctl client import`.
type ExportEntry struct {
	Name string   `json:"name"`
	IDs  []string `json:"ids"`
	Tags []string `json:"tags,omitempty"`
}

func exportCmd() *cobra.Command {
	var adguard, hosts, all bool
	c := &cobra.Command{
		Use: "export", Short: "Device inventory as JSON (--adguard) or a hosts file (--hosts)",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctx()
			defer cancel()
			id, err := netID(ctx)
			if err != nil {
				return err
			}
			devs, err := client.Devices(ctx, id)
			if err != nil {
				return err
			}
			sortDevices(devs)
			switch {
			case hosts:
				for _, d := range devs {
					if d.IP != "" && (all || d.Connected) {
						fmt.Printf("%s\t%s\n", d.IP, hostname(d.Name()))
					}
				}
				return nil
			default:
				var out []ExportEntry
				for _, d := range devs {
					if !all && !d.Connected {
						continue
					}
					ids := []string{}
					if d.IP != "" {
						ids = append(ids, d.IP)
					}
					if d.MAC != "" {
						ids = append(ids, d.MAC)
					}
					e := ExportEntry{Name: d.Name(), IDs: ids}
					if adguard && d.DeviceType != "" {
						e.Tags = []string{adguardTag(d.DeviceType)}
					}
					out = append(out, e)
				}
				return emit(out)
			}
		},
	}
	c.Flags().BoolVar(&adguard, "adguard", false, "add AdGuard Home client tags from the device type")
	c.Flags().BoolVar(&hosts, "hosts", false, "print ip<TAB>hostname lines")
	c.Flags().BoolVar(&all, "all", false, "include offline devices")
	return c
}

func hostname(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '_', r == '-', r == '.':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// adguardTag maps eero device types onto AdGuard Home's fixed client tag set.
func adguardTag(t string) string {
	switch strings.ToLower(t) {
	case "phone", "smartphone":
		return "device_phone"
	case "tablet":
		return "device_tablet"
	case "laptop":
		return "device_laptop"
	case "computer", "desktop", "pc":
		return "device_pc"
	case "tv", "smart_tv", "streaming", "media_player":
		return "device_tv"
	case "game_console":
		return "device_gameconsole"
	case "printer":
		return "device_printer"
	case "camera", "security_camera":
		return "device_securityalarm"
	case "nas", "server":
		return "device_nas"
	case "audio", "speaker", "voice_assistant":
		return "device_audio"
	}
	return "device_other"
}

func rawCmd() *cobra.Command {
	var method, body string
	c := &cobra.Command{
		Use: "raw <path>", Short: "Call any API path, e.g. networks/123/devices or /2.2/account", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctx()
			defer cancel()
			var b any
			if body != "" {
				if err := json.Unmarshal([]byte(body), &b); err != nil {
					return fmt.Errorf("--body is not JSON: %w", err)
				}
			}
			out, err := client.Raw(ctx, method, args[0], b)
			if err != nil {
				return err
			}
			fmt.Println(eero.Pretty(out))
			return nil
		},
	}
	c.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method")
	c.Flags().StringVarP(&body, "body", "d", "", "JSON body")
	return c
}
