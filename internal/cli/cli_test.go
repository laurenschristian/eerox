package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/laurenschristian/eerox/internal/eero"
)

func ok(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"code": 200}, "data": data})
}

var devices = []map[string]any{
	{"url": "/2.2/networks/9/devices/a1", "mac": "aa:bb", "ip": "10.0.0.5", "hostname": "nas", "connected": true, "device_type": "nas", "wireless": false, "source": map[string]any{"location": "Office"}},
	{"url": "/2.2/networks/9/devices/b2", "mac": "cc:dd", "ip": "10.0.0.6", "nickname": "Kids iPad", "connected": true, "device_type": "tablet", "wireless": true, "paused": true},
	{"url": "/2.2/networks/9/devices/c3", "mac": "ee:ff", "ip": "", "nickname": "Old Phone", "connected": false, "device_type": "phone", "wireless": true},
}

func setup(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var puts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/2.2/account":
			ok(w, map[string]any{"name": "L", "networks": map[string]any{"count": 1, "data": []map[string]any{{"url": "/2.2/networks/9", "name": "Home"}}}})
		case r.URL.Path == "/2.2/networks/9":
			ok(w, map[string]any{"url": "/2.2/networks/9", "name": "Home", "wan_ip": "1.2.3.4", "health": map[string]any{"internet": map[string]any{"status": "connected", "isp_up": true}, "eero_network": map[string]any{"status": "connected"}}, "eeros": map[string]any{"count": 2}})
		case r.URL.Path == "/2.2/networks/9/devices":
			ok(w, devices)
		case r.URL.Path == "/2.2/networks/9/devices/b2" && r.Method == http.MethodGet:
			ok(w, devices[1])
		case r.URL.Path == "/2.2/networks/9/devices/b2" && r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			puts = append(puts, string(b))
			ok(w, nil)
		case r.URL.Path == "/2.2/networks/9/eeros":
			ok(w, []map[string]any{{"url": "/2.2/eeros/e1", "location": "Office", "gateway": true, "model": "Pro 6E", "ip_address": "10.0.0.1", "status": "green", "mesh_quality_bars": 5}})
		case r.URL.Path == "/2.2/networks/9/profiles":
			ok(w, []map[string]any{{"url": "/2.2/networks/9/profiles/p1", "name": "Kids", "paused": false}})
		case r.URL.Path == "/2.2/networks/9/profiles/p1" && r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			puts = append(puts, "profile:"+string(b))
			ok(w, nil)
		case r.URL.Path == "/2.2/networks/9/reservations":
			ok(w, []map[string]any{{"url": "/2.2/networks/9/reservations/r1", "ip": "10.0.0.5", "mac": "aa:bb", "description": "nas"}})
		case r.URL.Path == "/2.2/networks/9/reservations/r1" && r.Method == http.MethodDelete:
			puts = append(puts, "delete:r1")
			ok(w, nil)
		case r.URL.Path == "/2.2/networks/9/forwards":
			ok(w, []map[string]any{{"url": "/2.2/networks/9/forwards/f1", "ip": "10.0.0.5", "protocol": "tcp", "gateway_port": 443, "client_port": 443, "enabled": true}})
		case r.URL.Path == "/2.2/networks/9/guestnetwork":
			if r.Method == http.MethodPut {
				b, _ := io.ReadAll(r.Body)
				puts = append(puts, "guest:"+string(b))
			}
			ok(w, map[string]any{"name": "Guests", "enabled": true, "password": "pw"})
		case r.URL.Path == "/2.2/networks/9/speedtest":
			ok(w, map[string]any{"up": map[string]any{"value": 40, "units": "Mbps"}, "down": map[string]any{"value": 900, "units": "Mbps"}})
		case r.URL.Path == "/2.2/eeros/e1/reboot":
			puts = append(puts, "reboot:e1")
			ok(w, nil)
		case r.URL.Path == "/2.2/networks/9/reboot":
			puts = append(puts, "reboot:network")
			ok(w, nil)
		case r.URL.Path == "/2.2/logout":
			ok(w, nil)
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"code": 404, "error": "error.not_found"}})
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("EERO_BASE", srv.URL)
	t.Setenv("EERO_TOKEN", "tok")
	t.Setenv("EERO_NETWORK", "")
	t.Setenv("EEROX_CONFIG", filepath.Join(t.TempDir(), "c.yaml"))
	return srv, &puts
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	root := Root()
	root.SetArgs(args)
	err := root.Execute()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), err
}

func TestDevicesTableAndFilters(t *testing.T) {
	setup(t)
	out, err := run(t, "devices")
	if err != nil || !strings.Contains(out, "nas") || !strings.Contains(out, "3 devices") {
		t.Fatalf("%v\n%s", err, out)
	}
	out, _ = run(t, "devices", "--online")
	if strings.Contains(out, "Old Phone") || !strings.Contains(out, "2 devices") {
		t.Fatalf("online filter\n%s", out)
	}
	out, _ = run(t, "devices", "--paused", "--json")
	var got []eero.Device
	_ = json.Unmarshal([]byte(out), &got)
	if len(got) != 1 || got[0].Name() != "Kids iPad" {
		t.Fatalf("paused json %s", out)
	}
	out, _ = run(t, "devices", "phone", "--offline", "--wireless")
	if !strings.Contains(out, "Old Phone") || !strings.Contains(out, "1 devices") {
		t.Fatalf("search\n%s", out)
	}
}

func TestNetworkDiscoveredAndCached(t *testing.T) {
	setup(t)
	out, err := run(t, "status")
	if err != nil || !strings.Contains(out, "Home: internet connected") || !strings.Contains(out, "2/3 devices online") {
		t.Fatalf("%v\n%s", err, out)
	}
	b, _ := os.ReadFile(os.Getenv("EEROX_CONFIG"))
	if !strings.Contains(string(b), `network: "9"`) {
		t.Fatalf("network not cached: %s", b)
	}
	out, _ = run(t, "network")
	if !strings.Contains(out, "wan ip") || !strings.Contains(out, "1.2.3.4") {
		t.Fatalf("network\n%s", out)
	}
	out, _ = run(t, "networks")
	if !strings.Contains(out, "Home") {
		t.Fatalf("networks\n%s", out)
	}
	out, _ = run(t, "account", "--json")
	if !strings.Contains(out, `"name": "L"`) {
		t.Fatalf("account\n%s", out)
	}
}

func TestDeviceMutations(t *testing.T) {
	_, puts := setup(t)
	for _, args := range [][]string{
		{"device", "rename", "Kids iPad", "Ipad Mini"},
		{"device", "pause", "10.0.0.6"},
		{"device", "unpause", "cc:dd"},
		{"device", "block", "b2"},
		{"device", "unblock", "ipad"},
	} {
		if out, err := run(t, args...); err != nil || !strings.Contains(out, "ok: Kids iPad (b2)") {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	want := []string{`{"nickname":"Ipad Mini"}`, `{"paused":true}`, `{"paused":false}`, `{"blacklisted":true}`, `{"blacklisted":false}`}
	for i, w := range want {
		if strings.TrimSpace((*puts)[i]) != w {
			t.Fatalf("put %d = %s want %s", i, (*puts)[i], w)
		}
	}
	out, err := run(t, "device", "b2")
	if err != nil || !strings.Contains(out, "Kids iPad") || !strings.Contains(out, "paused") {
		t.Fatalf("%v\n%s", err, out)
	}
	if _, err := run(t, "device", "nothing"); err == nil {
		t.Fatal("want not found")
	}
}

func TestEerosProfilesReservationsForwardsGuestSpeed(t *testing.T) {
	_, puts := setup(t)
	out, _ := run(t, "eeros")
	if !strings.Contains(out, "Pro 6E") || !strings.Contains(out, "gateway") {
		t.Fatalf("eeros\n%s", out)
	}
	if _, err := run(t, "reboot", "office"); err == nil {
		t.Fatal("reboot must need --yes")
	}
	if _, err := run(t, "reboot", "office", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "reboot", "--yes"); err != nil {
		t.Fatal(err)
	}
	out, _ = run(t, "profiles")
	if !strings.Contains(out, "Kids") {
		t.Fatalf("profiles\n%s", out)
	}
	if _, err := run(t, "profile", "pause", "kids"); err != nil {
		t.Fatal(err)
	}
	out, _ = run(t, "reservations")
	if !strings.Contains(out, "10.0.0.5") {
		t.Fatalf("reservations\n%s", out)
	}
	if _, err := run(t, "reservations", "rm", "aa:bb"); err != nil {
		t.Fatal(err)
	}
	out, _ = run(t, "forwards")
	if !strings.Contains(out, "443") {
		t.Fatalf("forwards\n%s", out)
	}
	out, _ = run(t, "guest", "--off", "--password", "newpw")
	if !strings.Contains(out, "Guests") {
		t.Fatalf("guest\n%s", out)
	}
	out, _ = run(t, "speedtest")
	if !strings.Contains(out, "down 900.0 Mbps") {
		t.Fatalf("speedtest\n%s", out)
	}
	joined := strings.Join(*puts, "\n")
	for _, w := range []string{"reboot:e1", "reboot:network", `profile:{"paused":true}`, "delete:r1", `"enabled":false`, `"password":"newpw"`} {
		if !strings.Contains(joined, w) {
			t.Fatalf("missing %s in\n%s", w, joined)
		}
	}
}

func TestExport(t *testing.T) {
	setup(t)
	out, err := run(t, "export", "--adguard")
	if err != nil {
		t.Fatal(err)
	}
	var got []ExportEntry
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 2 {
		t.Fatalf("%v %s", err, out)
	}
	if got[0].Name != "Kids iPad" || got[0].Tags[0] != "device_tablet" || got[0].IDs[0] != "10.0.0.6" {
		t.Fatalf("%+v", got[0])
	}
	out, _ = run(t, "export", "--hosts", "--all")
	if !strings.Contains(out, "10.0.0.5\tnas") || strings.Contains(out, "old-phone") {
		t.Fatalf("hosts\n%s", out)
	}
	out, _ = run(t, "export", "--all")
	_ = json.Unmarshal([]byte(out), &got)
	if len(got) != 3 || len(got[2].IDs) != 1 {
		t.Fatalf("all: %+v", got)
	}
}

func TestExportName(t *testing.T) {
	cases := []struct {
		d    eero.Device
		want string
	}{
		{eero.Device{Nickname: "Kids iPad", Hostname: "wlan0"}, "Kids iPad"},
		{eero.Device{Hostname: "wlan0", Manufacturer: "Tuya Smart Inc.", MAC: "50:8a:06:65:bc:21"}, "Tuya bc21"},
		{eero.Device{Hostname: "lwip0", MAC: "80:64:7c:ea:00:50"}, "Device 0050"},
		{eero.Device{MAC: "aa:bb:cc:dd:ee:ff"}, "Device eeff"},
		{eero.Device{Hostname: "Tuya Smart Inc.", Manufacturer: "Tuya Smart Inc.", MAC: "00:11"}, "Tuya 0011"},
		{eero.Device{Hostname: "nas"}, "nas"},
	}
	for _, c := range cases {
		if got := exportName(c.d); got != c.want {
			t.Errorf("%+v: got %q want %q", c.d, got, c.want)
		}
	}
}

func TestRenameBatch(t *testing.T) {
	_, puts := setup(t)
	f := filepath.Join(t.TempDir(), "names.txt")
	_ = os.WriteFile(f, []byte("# comment\n10.0.0.5\tNAS | Office\nKids iPad=Kids iPad\nzzz\tNope\nbroken line\naa:bb\tNAS | Office\n"), 0o600)
	out, err := run(t, "rename-batch", f, "--dry-run")
	if err != nil || !strings.Contains(strings.Join(strings.Fields(out), " "), "nas -> NAS | Office") || !strings.Contains(out, "renamed 1, unchanged 2, skipped 2 (dry run)") {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(*puts) != 0 {
		t.Fatal("dry run wrote")
	}
	_ = os.WriteFile(f, []byte("b2\tKids iPad Mini\n"), 0o600)
	out, err = run(t, "rename-batch", f)
	if err != nil || !strings.Contains(out, "renamed 1, unchanged 0, skipped 0") || !strings.Contains((*puts)[0], `"nickname":"Kids iPad Mini"`) {
		t.Fatalf("%v\n%s\n%v", err, out, *puts)
	}
	if _, err := run(t, "rename-batch", "/nonexistent"); err == nil {
		t.Fatal("missing file")
	}
}

func TestRawAndLogout(t *testing.T) {
	setup(t)
	out, err := run(t, "raw", "networks/9/eeros")
	if err != nil || !strings.Contains(out, "Pro 6E") {
		t.Fatalf("%v\n%s", err, out)
	}
	if _, err := run(t, "raw", "-X", "POST", "-d", "{bad", "x"); err == nil {
		t.Fatal("want body error")
	}
	if _, err := run(t, "raw", "nope"); err == nil {
		t.Fatal("want 404")
	}
	if _, err := run(t, "logout"); err != nil {
		t.Fatal(err)
	}
}

func TestNotLoggedIn(t *testing.T) {
	setup(t)
	t.Setenv("EERO_TOKEN", "")
	for _, args := range [][]string{{"completion", "zsh"}, {"help"}, {"--help"}} {
		if _, err := run(t, args...); err != nil {
			t.Fatalf("%v must work logged out: %v", args, err)
		}
	}
	if _, err := run(t, "devices"); err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("got %v", err)
	}
}

func TestHelpers(t *testing.T) {
	if hostname("Laurens' MacBook Pro.local") != "laurens-macbook-pro-local" {
		t.Fatal(hostname("Laurens' MacBook Pro.local"))
	}
	for in, want := range map[string]string{"phone": "device_phone", "tablet": "device_tablet", "laptop": "device_laptop", "computer": "device_pc", "tv": "device_tv", "game_console": "device_gameconsole", "printer": "device_printer", "camera": "device_securityalarm", "nas": "device_nas", "speaker": "device_audio", "toaster": "device_other"} {
		if adguardTag(in) != want {
			t.Errorf("%s -> %s", in, adguardTag(in))
		}
	}
	now := time.Now()
	if ago(now.Format(time.RFC3339)) != "now" || ago(now.Add(-5*time.Minute).Format(time.RFC3339)) != "5m" ||
		ago(now.Add(-3*time.Hour).Format(time.RFC3339)) != "3h" || ago(now.Add(-72*time.Hour).Format(time.RFC3339)) != "3d" || ago("junk") != "junk" {
		t.Fatal("ago")
	}
	d := eero.Device{Wireless: true}
	d.Interface.Frequency, d.Interface.FrequencyUnit, d.Connectivity.ScoreBars = "5", "GHz", 4
	if link(d) != "5GHz 4/5" || link(eero.Device{}) != "wired" {
		t.Fatal(link(d))
	}
	if deviceState(eero.Device{Blacklisted: true}) != "blocked" || deviceState(eero.Device{}) != "offline" {
		t.Fatal("state")
	}
}

func TestMCPTools(t *testing.T) {
	srv, puts := setup(t)
	c := eero.New(srv.URL, "tok")
	net := func(context.Context) (string, error) { return "9", nil }
	s := mcpServer(c, net)
	ct, st := mcp.NewInMemoryTransports()
	go func() { _ = s.Run(context.Background(), st) }()
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	tools, err := sess.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 19 {
		t.Fatalf("%v tools=%d", err, len(tools.Tools))
	}
	call := func(name string, args map[string]any) string {
		res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			return "ERR:" + res.Content[0].(*mcp.TextContent).Text
		}
		return res.Content[0].(*mcp.TextContent).Text
	}
	if out := call("eero_devices", map[string]any{"online": true}); strings.Contains(out, "Old Phone") || !strings.Contains(out, "nas") {
		t.Fatalf("devices %s", out)
	}
	if out := call("eero_rename_device", map[string]any{"device": "ipad", "name": "New"}); !strings.Contains(out, "ok: Kids iPad") {
		t.Fatalf("rename %s", out)
	}
	if out := call("eero_pause_device", map[string]any{"device": "b2", "on": true}); !strings.Contains(out, "ok") {
		t.Fatalf("pause %s", out)
	}
	if out := call("eero_device", map[string]any{"device": "zzz"}); !strings.HasPrefix(out, "ERR:") {
		t.Fatalf("want error, got %s", out)
	}
	call("eero_pause_profile", map[string]any{"profile": "Kids", "paused": true})
	call("eero_delete_reservation", map[string]any{"id": "10.0.0.5"})
	call("eero_set_guest_network", map[string]any{"enabled": false})
	call("eero_reboot", map[string]any{"eero": "Office"})
	for _, name := range []string{"eero_account", "eero_network", "eero_eeros", "eero_profiles", "eero_reservations", "eero_forwards", "eero_guest_network", "eero_speedtest"} {
		if out := call(name, nil); strings.HasPrefix(out, "ERR:") {
			t.Fatalf("%s: %s", name, out)
		}
	}
	if out := call("eero_raw", map[string]any{"path": "networks/9/eeros"}); !strings.Contains(out, "Pro 6E") {
		t.Fatalf("raw %s", out)
	}
	joined := strings.Join(*puts, "\n")
	for _, w := range []string{`"nickname":"New"`, `"paused":true`, "delete:r1", `"enabled":false`, "reboot:e1"} {
		if !strings.Contains(joined, w) {
			t.Fatalf("missing %s in\n%s", w, joined)
		}
	}
}

func TestLoginFlow(t *testing.T) {
	srv, _ := setup(t)
	t.Setenv("EERO_TOKEN", "")
	mux := http.NewServeMux()
	mux.HandleFunc("/2.2/login", func(w http.ResponseWriter, _ *http.Request) { ok(w, map[string]string{"user_token": "fresh"}) })
	mux.HandleFunc("/2.2/login/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "s=fresh" {
			t.Errorf("cookie %q", r.Header.Get("Cookie"))
		}
		ok(w, nil)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		r.URL.Scheme = "http"
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
		req.Header = r.Header
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(w, resp.Body)
	})
	login := httptest.NewServer(mux)
	defer login.Close()
	t.Setenv("EERO_BASE", login.URL)

	r, w, _ := os.Pipe()
	_, _ = w.WriteString("me@x.io\n123456\n")
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	out, err := run(t, "login", "--no-keychain")
	if err != nil || !strings.Contains(out, "logged in as me@x.io, network 9") {
		t.Fatalf("%v\n%s", err, out)
	}
	b, _ := os.ReadFile(os.Getenv("EEROX_CONFIG"))
	if !strings.Contains(string(b), "token: fresh") || !strings.Contains(string(b), "login: me@x.io") {
		t.Fatalf("config %s", b)
	}
	out, err = run(t, "devices", "--online")
	if err != nil || !strings.Contains(out, "2 devices") {
		t.Fatalf("after login: %v\n%s", err, out)
	}
}
