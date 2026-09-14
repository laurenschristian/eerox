package eero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fake(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL, "tok")
}

func ok(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"code": 200}, "data": data})
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"meta": map[string]any{"code": code, "error": msg}})
}

func TestLoginVerifyPersistsToken(t *testing.T) {
	var seenCookie string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2.2/login":
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["login"] != "me@x.io" {
				t.Errorf("login body %v", in)
			}
			ok(w, map[string]string{"user_token": "fresh"})
		case "/2.2/login/verify":
			seenCookie = r.Header.Get("Cookie")
			ok(w, nil)
		default:
			fail(w, 404, "nope")
		}
	})
	var saved string
	c.OnToken = func(s string) error { saved = s; return nil }
	tok, err := c.Login(context.Background(), "me@x.io")
	if err != nil || tok != "fresh" {
		t.Fatalf("login: %v %q", err, tok)
	}
	if err := c.Verify(context.Background(), "123456"); err != nil {
		t.Fatal(err)
	}
	if seenCookie != "s=fresh" || saved != "fresh" {
		t.Fatalf("cookie %q saved %q", seenCookie, saved)
	}
}

func TestRefreshOnStaleSession(t *testing.T) {
	calls := 0
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/2.2/account":
			calls++
			if r.Header.Get("Cookie") == "s=tok" {
				fail(w, 401, "error.session.refresh")
				return
			}
			ok(w, map[string]any{"name": "L"})
		case "/2.2/login/refresh":
			ok(w, map[string]string{"user_token": "tok2"})
		}
	})
	var saved string
	c.OnToken = func(s string) error { saved = s; return nil }
	a, err := c.Account(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "L" || saved != "tok2" || calls != 2 {
		t.Fatalf("name %q saved %q calls %d", a.Name, saved, calls)
	}
}

func TestAPIErrorAndPathNormalization(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.2/networks/1/devices" {
			t.Errorf("path %s", r.URL.Path)
		}
		fail(w, 403, "error.forbidden")
	})
	_, err := c.Raw(context.Background(), "GET", "networks/1/devices", nil)
	var ae *APIError
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want api error, got %v", err)
	}
	if !errorsAs(err, &ae) || ae.Message != "error.forbidden" {
		t.Fatalf("want *APIError, got %T", err)
	}
	_, err = c.Raw(context.Background(), "GET", "/2.2/networks/1/devices", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func errorsAs(err error, target **APIError) bool {
	ae, ok := err.(*APIError)
	if ok {
		*target = ae
	}
	return ok
}

func TestNonJSONResponse(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	})
	_, err := c.Account(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("got %v", err)
	}
}

func TestDevicesAndMutations(t *testing.T) {
	var put map[string]any
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/2.2/networks/9/devices" && r.Method == http.MethodGet:
			ok(w, []map[string]any{
				{"url": "/2.2/networks/9/devices/a1", "mac": "aa:bb", "ip": "10.0.0.5", "hostname": "nas", "connected": true},
				{"url": "/2.2/networks/9/devices/b2", "mac": "cc:dd", "ip": "10.0.0.6", "nickname": "Kids iPad", "connected": false},
			})
		case r.URL.Path == "/2.2/networks/9/devices/b2" && r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&put)
			ok(w, nil)
		case r.URL.Path == "/2.2/networks/9/reservations" && r.Method == http.MethodPost:
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			ok(w, map[string]string{"url": "/2.2/networks/9/reservations/77", "ip": in["ip"], "mac": in["mac"]})
		case r.URL.Path == "/2.2/networks/9/speedtest" && r.Method == http.MethodPost:
			ok(w, map[string]any{"up": map[string]any{"value": 40.5, "units": "Mbps"}, "down": map[string]any{"value": 900, "units": "Mbps"}})
		default:
			fail(w, 404, "nope")
		}
	})
	ctx := context.Background()
	devs, err := c.Devices(ctx, "9")
	if err != nil || len(devs) != 2 {
		t.Fatalf("%v %d", err, len(devs))
	}
	if devs[0].ID() != "a1" || devs[0].Name() != "nas" || devs[1].Name() != "Kids iPad" {
		t.Fatalf("ids/names wrong: %+v", devs)
	}
	if err := c.UpdateDevice(ctx, "9", "b2", map[string]any{"paused": true}); err != nil || put["paused"] != true {
		t.Fatalf("update %v %v", err, put)
	}
	r, err := c.AddReservation(ctx, "9", "10.0.0.7", "AA:BB:CC:DD:EE:FF", "printer")
	if err != nil || r.ID() != "77" || r.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("reservation %v %+v", err, r)
	}
	st, err := c.SpeedTest(ctx, "9")
	if err != nil || st.Down.Value != 900 {
		t.Fatalf("speedtest %v %+v", err, st)
	}
}

func TestFindDevice(t *testing.T) {
	devs := []Device{
		{URL: "/x/devices/1", MAC: "aa:bb", IP: "10.0.0.1", Hostname: "office-mac"},
		{URL: "/x/devices/2", MAC: "cc:dd", IP: "10.0.0.2", Nickname: "Office Printer"},
		{URL: "/x/devices/3", MAC: "ee:ff", IP: "10.0.0.3", Nickname: "TV"},
	}
	for q, want := range map[string]string{"1": "1", "10.0.0.2": "2", "CC:DD": "2", "office-mac": "1", "tv": "3", "printer": "2"} {
		d, err := FindDevice(devs, q)
		if err != nil || d.ID() != want {
			t.Errorf("%q: got %v %v, want %s", q, d, err, want)
		}
	}
	if _, err := FindDevice(devs, "office"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("want ambiguous, got %v", err)
	}
	if _, err := FindDevice(devs, "zzz"); err == nil {
		t.Error("want not found")
	}
}

func TestIDAndPretty(t *testing.T) {
	if ID("/2.2/networks/12345") != "12345" || ID("abc") != "abc" {
		t.Fatal("ID")
	}
	if got := Pretty(json.RawMessage(`{"a":1}`)); got != "{\n  \"a\": 1\n}" {
		t.Fatalf("pretty %q", got)
	}
	if got := Pretty(json.RawMessage(`nope`)); got != "nope" {
		t.Fatalf("pretty passthrough %q", got)
	}
}

func TestReadEndpointsAndActions(t *testing.T) {
	var hits []string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/2.2/networks/9":
			ok(w, map[string]any{"url": "/2.2/networks/9", "name": "Home", "wan_ip": "1.2.3.4"})
		case "/2.2/networks/9/devices/a1":
			ok(w, map[string]any{"url": "/2.2/networks/9/devices/a1", "hostname": "nas"})
		case "/2.2/networks/9/eeros":
			ok(w, []map[string]any{{"url": "/2.2/eeros/e1", "location": "Office"}})
		case "/2.2/networks/9/profiles":
			ok(w, []map[string]any{{"url": "/2.2/networks/9/profiles/p1", "name": "Kids"}})
		case "/2.2/networks/9/reservations":
			ok(w, []map[string]any{{"url": "/2.2/networks/9/reservations/r1", "ip": "10.0.0.5"}})
		case "/2.2/networks/9/forwards":
			ok(w, []map[string]any{{"url": "/2.2/networks/9/forwards/f1", "gateway_port": 443}})
		case "/2.2/networks/9/guestnetwork":
			ok(w, map[string]any{"name": "Guests", "enabled": true})
		default:
			ok(w, nil)
		}
	})
	ctx := context.Background()
	n, err := c.Network(ctx, "9")
	if err != nil || n.ID() != "9" || n.WanIP != "1.2.3.4" {
		t.Fatalf("network %v %+v", err, n)
	}
	d, err := c.Device(ctx, "9", "a1")
	if err != nil || d.Name() != "nas" || d.ProfileName() != "" {
		t.Fatalf("device %v %+v", err, d)
	}
	es, err := c.Eeros(ctx, "9")
	if err != nil || es[0].ID() != "e1" {
		t.Fatalf("eeros %v", err)
	}
	ps, err := c.Profiles(ctx, "9")
	if err != nil || ps[0].ID() != "p1" {
		t.Fatalf("profiles %v", err)
	}
	rs, err := c.Reservations(ctx, "9")
	if err != nil || rs[0].ID() != "r1" {
		t.Fatalf("reservations %v", err)
	}
	fs, err := c.Forwards(ctx, "9")
	if err != nil || fs[0].ID() != "f1" || fs[0].GatewayPort != 443 {
		t.Fatalf("forwards %v", err)
	}
	g, err := c.GuestNetwork(ctx, "9")
	if err != nil || g.Name != "Guests" {
		t.Fatalf("guest %v", err)
	}
	for _, fn := range []func() error{
		func() error { return c.RebootEero(ctx, "e1") },
		func() error { return c.RebootNetwork(ctx, "9") },
		func() error { return c.UpdateProfile(ctx, "9", "p1", map[string]any{"paused": true}) },
		func() error { return c.DeleteReservation(ctx, "9", "r1") },
		func() error { return c.UpdateGuestNetwork(ctx, "9", map[string]any{"enabled": false}) },
		func() error { return c.Logout(ctx) },
	} {
		if err := fn(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"POST /2.2/eeros/e1/reboot", "POST /2.2/networks/9/reboot", "PUT /2.2/networks/9/profiles/p1", "DELETE /2.2/networks/9/reservations/r1", "PUT /2.2/networks/9/guestnetwork", "POST /2.2/logout"}
	got := strings.Join(hits, "\n")
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %s in\n%s", w, got)
		}
	}
}

func TestRefreshFailurePropagates(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/2.2/login/refresh" {
			fail(w, 401, "error.session.invalid")
			return
		}
		fail(w, 401, "error.session.refresh")
	})
	c.OnToken = func(string) error { return nil }
	if _, err := c.Account(context.Background()); err == nil || !strings.Contains(err.Error(), "eerox login") {
		t.Fatalf("got %v", err)
	}
	c2 := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/2.2/login/refresh" {
			ok(w, map[string]any{})
			return
		}
		fail(w, 401, "error.session.refresh")
	})
	if _, err := c2.Account(context.Background()); err == nil || !strings.Contains(err.Error(), "no token") {
		t.Fatalf("got %v", err)
	}
}

func TestDefaultBase(t *testing.T) {
	if New("", "").Base != DefaultBase || New("http://x/", "").Base != "http://x" {
		t.Fatal("base")
	}
}
