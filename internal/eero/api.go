package eero

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Account struct {
	Name  string `json:"name"`
	Email struct {
		Value    string `json:"value"`
		Verified bool   `json:"verified"`
	} `json:"email"`
	Phone struct {
		Value    string `json:"value"`
		Verified bool   `json:"verified"`
	} `json:"phone"`
	PremiumStatus string `json:"premium_status"`
	Networks      struct {
		Count int          `json:"count"`
		Data  []NetworkRef `json:"data,omitempty"`
	} `json:"networks"`
}

type NetworkRef struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Created string `json:"created"`
}

func (n NetworkRef) ID() string { return ID(n.URL) }

type Network struct {
	URL        string `json:"url"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	WanIP      string `json:"wan_ip"`
	GatewayIP  string `json:"gateway_ip"`
	WanType    string `json:"wan_type"`
	Connection struct {
		Mode string `json:"mode"`
	} `json:"connection"`
	GeoIP struct {
		ISP  string `json:"isp"`
		Org  string `json:"org"`
		City string `json:"city"`
	} `json:"geo_ip"`
	Lease struct {
		Mode string `json:"mode"`
		DHCP *struct {
			IP     string `json:"ip"`
			Mask   string `json:"mask"`
			Router string `json:"router"`
		} `json:"dhcp"`
	} `json:"lease"`
	DHCP struct {
		Mode   string `json:"mode"`
		Custom *struct {
			SubnetIP   string `json:"subnet_ip"`
			SubnetMask string `json:"subnet_mask"`
			StartIP    string `json:"start_ip"`
			EndIP      string `json:"end_ip"`
		} `json:"custom"`
	} `json:"dhcp"`
	DNS struct {
		Mode    string `json:"mode"`
		Caching bool   `json:"caching"`
		Custom  *struct {
			IPs []string `json:"ips,omitempty"`
		} `json:"custom"`
		Parent struct {
			IPs []string `json:"ips,omitempty"`
		} `json:"parent"`
	} `json:"dns"`
	Upnp         bool   `json:"upnp"`
	IPv6Upstream bool   `json:"ipv6_upstream"`
	SQM          bool   `json:"sqm"`
	BandSteering bool   `json:"band_steering"`
	Wpa3         bool   `json:"wpa3"`
	WirelessMode string `json:"wireless_mode"`
	LastReboot   string `json:"last_reboot"`
	Speed        struct {
		Status string `json:"status"`
		Date   string `json:"date"`
		Up     Speed  `json:"up"`
		Down   Speed  `json:"down"`
	} `json:"speed"`
	Health struct {
		Internet struct {
			Status string `json:"status"`
			ISPUp  bool   `json:"isp_up"`
		} `json:"internet"`
		EeroNetwork struct {
			Status string `json:"status"`
		} `json:"eero_network"`
	} `json:"health"`
	Updates struct {
		TargetFirmware string `json:"target_firmware"`
		HasUpdate      bool   `json:"has_update"`
		UpdateRequired bool   `json:"update_required"`
		CanUpdateNow   bool   `json:"can_update_now"`
	} `json:"updates"`
	Eeros struct {
		Count int    `json:"count"`
		Data  []Eero `json:"data,omitempty"`
	} `json:"eeros"`
	GuestNetwork struct {
		URL     string `json:"url"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"guest_network"`
	PremiumStatus string `json:"premium_status"`
	Owner         string `json:"owner"`
}

type Speed struct {
	Value float64 `json:"value"`
	Units string  `json:"units"`
}

type Eero struct {
	URL                   string   `json:"url"`
	Serial                string   `json:"serial"`
	Location              string   `json:"location"`
	Gateway               bool     `json:"gateway"`
	IPAddress             string   `json:"ip_address"`
	Status                string   `json:"status"`
	State                 string   `json:"state"`
	Model                 string   `json:"model"`
	ModelNumber           string   `json:"model_number"`
	OS                    string   `json:"os"`
	OSVersion             string   `json:"os_version"`
	MACAddress            string   `json:"mac_address"`
	EthernetAddresses     []string `json:"ethernet_addresses,omitempty"`
	Wired                 bool     `json:"wired"`
	UpdateAvailable       bool     `json:"update_available"`
	MeshQualityBars       int      `json:"mesh_quality_bars"`
	ConnectedClientsCount int      `json:"connected_clients_count"`
	HeartbeatOK           bool     `json:"heartbeat_ok"`
	LastHeartbeat         string   `json:"last_heartbeat"`
	ConnectionType        string   `json:"connection_type"`
	LedOn                 bool     `json:"led_on"`
	Bands                 []string `json:"bands,omitempty"`
	Joined                string   `json:"joined"`
}

func (e Eero) ID() string { return ID(e.URL) }

func (n Network) ID() string { return ID(n.URL) }

type Device struct {
	URL            string   `json:"url"`
	MAC            string   `json:"mac"`
	IP             string   `json:"ip"`
	IPs            []string `json:"ips,omitempty"`
	Nickname       string   `json:"nickname"`
	Hostname       string   `json:"hostname"`
	DisplayName    string   `json:"display_name"`
	Manufacturer   string   `json:"manufacturer"`
	ModelName      string   `json:"model_name"`
	DeviceType     string   `json:"device_type"`
	Connected      bool     `json:"connected"`
	Wireless       bool     `json:"wireless"`
	ConnectionType string   `json:"connection_type"`
	Paused         bool     `json:"paused"`
	Blacklisted    bool     `json:"blacklisted"`
	IsGuest        bool     `json:"is_guest"`
	IsPrivate      bool     `json:"is_private"`
	LastActive     string   `json:"last_active"`
	FirstActive    string   `json:"first_active"`
	SSID           string   `json:"ssid"`
	Channel        int      `json:"channel"`
	Source         struct {
		Location    string `json:"location"`
		IsGateway   bool   `json:"is_gateway"`
		DisplayName string `json:"display_name"`
	} `json:"source"`
	Profile *struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	} `json:"profile"`
	Connectivity struct {
		Signal    string  `json:"signal"`
		Score     float64 `json:"score"`
		ScoreBars int     `json:"score_bars"`
		RxBitrate string  `json:"rx_bitrate"`
		Frequency int     `json:"frequency"`
	} `json:"connectivity"`
	Interface struct {
		Frequency     string `json:"frequency"`
		FrequencyUnit string `json:"frequency_unit"`
	} `json:"interface"`
	Usage *struct {
		Download float64 `json:"download"`
		Upload   float64 `json:"upload"`
		Units    string  `json:"units"`
	} `json:"usage"`
}

func (d Device) ID() string { return ID(d.URL) }

// Name is the best human label: nickname, then hostname, then display_name, then MAC.
func (d Device) Name() string {
	for _, s := range []string{d.Nickname, d.Hostname, d.DisplayName} {
		if s != "" {
			return s
		}
	}
	return d.MAC
}

func (d Device) ProfileName() string {
	if d.Profile != nil {
		return d.Profile.Name
	}
	return ""
}

type Profile struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Paused  bool   `json:"paused"`
	Default bool   `json:"default"`
	Devices []struct {
		URL      string `json:"url"`
		Nickname string `json:"nickname"`
		Hostname string `json:"hostname"`
		MAC      string `json:"mac"`
	} `json:"devices"`
}

func (p Profile) ID() string { return ID(p.URL) }

type Reservation struct {
	URL         string `json:"url"`
	IP          string `json:"ip"`
	MAC         string `json:"mac"`
	Description string `json:"description"`
}

func (r Reservation) ID() string { return ID(r.URL) }

type Forward struct {
	URL         string `json:"url"`
	IP          string `json:"ip"`
	Description string `json:"description"`
	Protocol    string `json:"protocol"`
	GatewayPort int    `json:"gateway_port"`
	ClientPort  int    `json:"client_port"`
	Enabled     bool   `json:"enabled"`
}

func (f Forward) ID() string { return ID(f.URL) }

type GuestNetwork struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Password string `json:"password"`
}

type SpeedTest struct {
	Up   Speed  `json:"up"`
	Down Speed  `json:"down"`
	Date string `json:"date"`
}

func net(id string) string { return "networks/" + id }

func (c *Client) Account(ctx context.Context) (*Account, error) {
	var a Account
	return &a, c.do(ctx, "GET", "account", nil, &a)
}

func (c *Client) Network(ctx context.Context, id string) (*Network, error) {
	var n Network
	return &n, c.do(ctx, "GET", net(id), nil, &n)
}

func (c *Client) Devices(ctx context.Context, id string) ([]Device, error) {
	var d []Device
	return d, c.do(ctx, "GET", net(id)+"/devices", nil, &d)
}

func (c *Client) Device(ctx context.Context, id, dev string) (*Device, error) {
	var d Device
	return &d, c.do(ctx, "GET", net(id)+"/devices/"+dev, nil, &d)
}

func (c *Client) UpdateDevice(ctx context.Context, id, dev string, fields map[string]any) error {
	return c.do(ctx, "PUT", net(id)+"/devices/"+dev, fields, nil)
}

func (c *Client) Eeros(ctx context.Context, id string) ([]Eero, error) {
	var e []Eero
	return e, c.do(ctx, "GET", net(id)+"/eeros", nil, &e)
}

func (c *Client) RebootEero(ctx context.Context, eero string) error {
	return c.do(ctx, "POST", "eeros/"+eero+"/reboot", nil, nil)
}

func (c *Client) RebootNetwork(ctx context.Context, id string) error {
	return c.do(ctx, "POST", net(id)+"/reboot", nil, nil)
}

func (c *Client) Profiles(ctx context.Context, id string) ([]Profile, error) {
	var p []Profile
	return p, c.do(ctx, "GET", net(id)+"/profiles", nil, &p)
}

func (c *Client) UpdateProfile(ctx context.Context, id, prof string, fields map[string]any) error {
	return c.do(ctx, "PUT", net(id)+"/profiles/"+prof, fields, nil)
}

func (c *Client) Reservations(ctx context.Context, id string) ([]Reservation, error) {
	var r []Reservation
	return r, c.do(ctx, "GET", net(id)+"/reservations", nil, &r)
}

func (c *Client) AddReservation(ctx context.Context, id, ip, mac, desc string) (*Reservation, error) {
	var r Reservation
	body := map[string]string{"ip": ip, "mac": strings.ToLower(mac), "description": desc}
	return &r, c.do(ctx, "POST", net(id)+"/reservations", body, &r)
}

func (c *Client) DeleteReservation(ctx context.Context, id, res string) error {
	return c.do(ctx, "DELETE", net(id)+"/reservations/"+res, nil, nil)
}

func (c *Client) Forwards(ctx context.Context, id string) ([]Forward, error) {
	var f []Forward
	return f, c.do(ctx, "GET", net(id)+"/forwards", nil, &f)
}

func (c *Client) GuestNetwork(ctx context.Context, id string) (*GuestNetwork, error) {
	var g GuestNetwork
	return &g, c.do(ctx, "GET", net(id)+"/guestnetwork", nil, &g)
}

func (c *Client) UpdateGuestNetwork(ctx context.Context, id string, fields map[string]any) error {
	return c.do(ctx, "PUT", net(id)+"/guestnetwork", fields, nil)
}

// SpeedTest runs a speed test from the gateway. It blocks for the test duration (~30s).
func (c *Client) SpeedTest(ctx context.Context, id string) (*SpeedTest, error) {
	var s SpeedTest
	return &s, c.do(ctx, "POST", net(id)+"/speedtest", nil, &s)
}

// FindDevice matches by id, ip, mac, nickname, hostname (case-insensitive), then substring of the name.
func FindDevice(devs []Device, q string) (*Device, error) {
	lq := strings.ToLower(q)
	for i := range devs {
		d := &devs[i]
		if d.ID() == q || strings.EqualFold(d.IP, q) || strings.EqualFold(d.MAC, q) ||
			strings.EqualFold(d.Nickname, q) || strings.EqualFold(d.Hostname, q) {
			return d, nil
		}
	}
	var hits []*Device
	for i := range devs {
		if strings.Contains(strings.ToLower(devs[i].Name()), lq) {
			hits = append(hits, &devs[i])
		}
	}
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("no device matches %q", q)
	case 1:
		return hits[0], nil
	}
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = fmt.Sprintf("%s (%s)", h.Name(), h.ID())
	}
	return nil, fmt.Errorf("%q is ambiguous: %s", q, strings.Join(names, ", "))
}

// Pretty re-encodes raw JSON with indentation.
func Pretty(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
