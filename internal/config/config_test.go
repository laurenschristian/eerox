package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadAndEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("EEROX_CONFIG", p)
	t.Setenv("EERO_TOKEN", "")
	t.Setenv("EERO_NETWORK", "")
	if err := Save(&Config{Login: "me@x.io", Network: "1", Token: "plain"}); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil || c.Token != "plain" || c.Network != "1" {
		t.Fatalf("%v %+v", err, c)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
	t.Setenv("EERO_TOKEN", "envtok")
	t.Setenv("EERO_NETWORK", "2")
	c, _ = Load()
	if c.Token != "envtok" || c.Network != "2" || c.Keychain {
		t.Fatalf("env override %+v", c)
	}
}

func TestKeychainModeNeverWritesToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("EEROX_CONFIG", p)
	if err := Save(&Config{Login: "me@x.io", Token: "secret", Keychain: true}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "secret") {
		t.Fatalf("token leaked to file: %s", b)
	}
}

func TestTokenCmd(t *testing.T) {
	c := &Config{TokenCmd: "echo fromcmd"}
	if err := c.ResolveToken(); err != nil || c.Token != "fromcmd" {
		t.Fatalf("%v %q", err, c.Token)
	}
	bad := &Config{TokenCmd: "exit 3"}
	if err := bad.ResolveToken(); err == nil {
		t.Fatal("want error")
	}
	keep := &Config{Token: "x", TokenCmd: "echo y"}
	_ = keep.ResolveToken()
	if keep.Token != "x" {
		t.Fatal("existing token must win")
	}
}

func TestPathDefault(t *testing.T) {
	t.Setenv("EEROX_CONFIG", "")
	if !strings.HasSuffix(Path(), filepath.Join("eerox", "config.yaml")) {
		t.Fatal(Path())
	}
}

func TestSaveAndClearTokenFileMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("EEROX_CONFIG", p)
	c := &Config{Login: "me@x.io"}
	if err := c.SaveToken("t1"); err != nil || c.Token != "t1" {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "token: t1") {
		t.Fatalf("file %s", b)
	}
	if err := c.ClearToken(); err != nil || c.Token != "" {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(p)
	if strings.Contains(string(b), "t1") {
		t.Fatalf("token still present: %s", b)
	}
	_ = KeychainAvailable()
}

func TestBadYAML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("EEROX_CONFIG", p)
	_ = os.WriteFile(p, []byte("login: [unclosed"), 0o600)
	if _, err := Load(); err == nil {
		t.Fatal("want yaml error")
	}
}
