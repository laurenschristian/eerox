package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/laurenschristian/eerox/internal/eero"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP server over stdio (for Claude, Cursor, etc.)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcpServer(client, netID).Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}

type devIn struct {
	Device string `json:"device" jsonschema:"device id, ip, mac, nickname or hostname"`
}
type renameIn struct {
	Device string `json:"device" jsonschema:"device id, ip, mac, nickname or hostname"`
	Name   string `json:"name" jsonschema:"new nickname"`
}
type boolDevIn struct {
	Device string `json:"device" jsonschema:"device id, ip, mac, nickname or hostname"`
	On     bool   `json:"on" jsonschema:"true to apply, false to lift"`
}
type filterIn struct {
	Search string `json:"search,omitempty" jsonschema:"substring of name, ip, mac or manufacturer"`
	Online *bool  `json:"online,omitempty" jsonschema:"true = connected only, false = disconnected only"`
}
type profileIn struct {
	Profile string `json:"profile" jsonschema:"profile name or id"`
	Paused  bool   `json:"paused"`
}
type reservationIn struct {
	IP          string `json:"ip"`
	MAC         string `json:"mac"`
	Description string `json:"description,omitempty"`
}
type idIn struct {
	ID string `json:"id" jsonschema:"reservation id, ip or mac"`
}
type eeroIn struct {
	Eero string `json:"eero" jsonschema:"eero id or location name; empty reboots the whole network"`
}
type guestIn struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	Password string `json:"password,omitempty"`
	Name     string `json:"name,omitempty"`
}
type rawIn struct {
	Method string `json:"method,omitempty" jsonschema:"HTTP method, default GET"`
	Path   string `json:"path" jsonschema:"API path, e.g. networks/123/devices"`
	Body   any    `json:"body,omitempty"`
}
type rawOut struct {
	Data any `json:"data"`
}
type msgOut struct {
	Message string `json:"message"`
}

func mcpServer(c *eero.Client, network func(context.Context) (string, error)) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "eerox", Version: Version}, nil)

	mcp.AddTool(s, &mcp.Tool{Name: "eero_account", Description: "Account owner, contact and the networks on it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, *eero.Account, error) {
			a, err := c.Account(ctx)
			return nil, a, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_network", Description: "Network details: WAN IP, ISP, DHCP, DNS, health, firmware, last speed test."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, *eero.Network, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			n, err := c.Network(ctx, id)
			return nil, n, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_devices", Description: "Client devices with name, ip, mac, node, connection state, profile. Filter by search or online."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in filterIn) (*mcp.CallToolResult, []eero.Device, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			devs, err := c.Devices(ctx, id)
			if err != nil {
				return nil, nil, err
			}
			f := deviceFilter{search: in.Search}
			if in.Online != nil {
				f.online, f.offline = *in.Online, !*in.Online
			}
			out := []eero.Device{}
			for _, d := range devs {
				if f.keep(d) {
					out = append(out, d)
				}
			}
			sortDevices(out)
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_device", Description: "Full detail for one device incl. signal, usage and node."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in devIn) (*mcp.CallToolResult, *eero.Device, error) {
			id, d, err := findDeviceWith(ctx, c, network, in.Device)
			if err != nil {
				return nil, nil, err
			}
			full, err := c.Device(ctx, id, d.ID())
			return nil, full, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_rename_device", Description: "Set a device nickname."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in renameIn) (*mcp.CallToolResult, msgOut, error) {
			return deviceUpdate(ctx, c, network, in.Device, map[string]any{"nickname": in.Name})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_pause_device", Description: "Pause (on=true) or resume (on=false) internet for a device."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in boolDevIn) (*mcp.CallToolResult, msgOut, error) {
			return deviceUpdate(ctx, c, network, in.Device, map[string]any{"paused": in.On})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_block_device", Description: "Block (on=true) or unblock (on=false) a device from the network."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in boolDevIn) (*mcp.CallToolResult, msgOut, error) {
			return deviceUpdate(ctx, c, network, in.Device, map[string]any{"blacklisted": in.On})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_eeros", Description: "Mesh nodes: location, model, ip, status, mesh quality, client count, firmware."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, []eero.Eero, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			e, err := c.Eeros(ctx, id)
			return nil, e, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_reboot", Description: "Reboot one eero by id or location, or the whole network when eero is empty. Disruptive: confirm with the user first."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in eeroIn) (*mcp.CallToolResult, msgOut, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, msgOut{}, err
			}
			if in.Eero == "" {
				return nil, msgOut{Message: "network reboot requested"}, c.RebootNetwork(ctx, id)
			}
			es, err := c.Eeros(ctx, id)
			if err != nil {
				return nil, msgOut{}, err
			}
			for _, e := range es {
				if e.ID() == in.Eero || equalFold(e.Location, in.Eero) {
					return nil, msgOut{Message: "rebooting " + e.Location}, c.RebootEero(ctx, e.ID())
				}
			}
			return nil, msgOut{}, fmt.Errorf("no eero matches %q", in.Eero)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_profiles", Description: "Family profiles with paused state and member devices."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, []eero.Profile, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			p, err := c.Profiles(ctx, id)
			return nil, p, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_pause_profile", Description: "Pause or resume every device in a profile."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in profileIn) (*mcp.CallToolResult, msgOut, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, msgOut{}, err
			}
			ps, err := c.Profiles(ctx, id)
			if err != nil {
				return nil, msgOut{}, err
			}
			for _, p := range ps {
				if p.ID() == in.Profile || equalFold(p.Name, in.Profile) {
					return nil, msgOut{Message: "ok"}, c.UpdateProfile(ctx, id, p.ID(), map[string]any{"paused": in.Paused})
				}
			}
			return nil, msgOut{}, fmt.Errorf("no profile matches %q", in.Profile)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_reservations", Description: "DHCP reservations (ip, mac, description)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, []eero.Reservation, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			r, err := c.Reservations(ctx, id)
			return nil, r, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_add_reservation", Description: "Reserve an IP for a MAC address."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in reservationIn) (*mcp.CallToolResult, *eero.Reservation, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			r, err := c.AddReservation(ctx, id, in.IP, in.MAC, in.Description)
			return nil, r, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_delete_reservation", Description: "Delete a DHCP reservation by id, ip or mac."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, msgOut, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, msgOut{}, err
			}
			rs, err := c.Reservations(ctx, id)
			if err != nil {
				return nil, msgOut{}, err
			}
			for _, r := range rs {
				if r.ID() == in.ID || r.IP == in.ID || equalFold(r.MAC, in.ID) {
					return nil, msgOut{Message: "deleted " + r.IP}, c.DeleteReservation(ctx, id, r.ID())
				}
			}
			return nil, msgOut{}, fmt.Errorf("no reservation matches %q", in.ID)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_forwards", Description: "Port forwarding rules."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, []eero.Forward, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			f, err := c.Forwards(ctx, id)
			return nil, f, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_guest_network", Description: "Guest wifi state: name, enabled, password."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, *eero.GuestNetwork, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			g, err := c.GuestNetwork(ctx, id)
			return nil, g, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_set_guest_network", Description: "Enable/disable guest wifi or change its name or password."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in guestIn) (*mcp.CallToolResult, *eero.GuestNetwork, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			fields := map[string]any{}
			if in.Enabled != nil {
				fields["enabled"] = *in.Enabled
			}
			if in.Password != "" {
				fields["password"] = in.Password
			}
			if in.Name != "" {
				fields["name"] = in.Name
			}
			if len(fields) > 0 {
				if err := c.UpdateGuestNetwork(ctx, id, fields); err != nil {
					return nil, nil, err
				}
			}
			g, err := c.GuestNetwork(ctx, id)
			return nil, g, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_speedtest", Description: "Run a speed test from the gateway. Takes about 30 seconds."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, *eero.SpeedTest, error) {
			id, err := network(ctx)
			if err != nil {
				return nil, nil, err
			}
			st, err := c.SpeedTest(ctx, id)
			return nil, st, err
		})

	mcp.AddTool(s, &mcp.Tool{Name: "eero_raw", Description: "Call any eero API path and return the data payload."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in rawIn) (*mcp.CallToolResult, rawOut, error) {
			if in.Method == "" {
				in.Method = "GET"
			}
			out, err := c.Raw(ctx, in.Method, in.Path, in.Body)
			if err != nil {
				return nil, rawOut{}, err
			}
			var v any
			_ = json.Unmarshal(out, &v)
			return nil, rawOut{Data: v}, nil
		})

	return s
}

func findDeviceWith(ctx context.Context, c *eero.Client, network func(context.Context) (string, error), q string) (string, *eero.Device, error) {
	id, err := network(ctx)
	if err != nil {
		return "", nil, err
	}
	devs, err := c.Devices(ctx, id)
	if err != nil {
		return "", nil, err
	}
	d, err := eero.FindDevice(devs, q)
	return id, d, err
}

func deviceUpdate(ctx context.Context, c *eero.Client, network func(context.Context) (string, error), q string, fields map[string]any) (*mcp.CallToolResult, msgOut, error) {
	id, d, err := findDeviceWith(ctx, c, network, q)
	if err != nil {
		return nil, msgOut{}, err
	}
	if err := c.UpdateDevice(ctx, id, d.ID(), fields); err != nil {
		return nil, msgOut{}, err
	}
	return nil, msgOut{Message: fmt.Sprintf("ok: %s (%s)", d.Name(), d.ID())}, nil
}
