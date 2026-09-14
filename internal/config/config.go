// Package config resolves settings: flags > env > config file. The session token
// lives in the macOS Keychain by default, or in the file (0600) with --no-keychain.
package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const keychainService = "eerox"

type Config struct {
	Login    string `yaml:"login,omitempty"`
	Network  string `yaml:"network,omitempty"`
	Token    string `yaml:"token,omitempty"`
	TokenCmd string `yaml:"token_cmd,omitempty"`
	Keychain bool   `yaml:"keychain,omitempty"`
}

func Path() string {
	if p := os.Getenv("EEROX_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "eerox", "config.yaml")
}

func Load() (*Config, error) {
	c := &Config{}
	if b, err := os.ReadFile(Path()); err == nil {
		if err := yaml.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	if v := os.Getenv("EERO_TOKEN"); v != "" {
		c.Token = v
		c.Keychain = false
	}
	if v := os.Getenv("EERO_NETWORK"); v != "" {
		c.Network = v
	}
	return c, nil
}

// ResolveToken fills Token from the keychain or token_cmd. An empty token is not an error here.
func (c *Config) ResolveToken() error {
	if c.Token != "" {
		return nil
	}
	switch {
	case c.TokenCmd != "":
		out, err := exec.CommandContext(context.Background(), "sh", "-c", c.TokenCmd).Output()
		if err != nil {
			return errors.New("token_cmd failed: " + err.Error())
		}
		c.Token = strings.TrimSpace(string(out))
	case c.Keychain && c.Login != "":
		out, err := exec.CommandContext(context.Background(), "security", "find-generic-password", "-s", keychainService, "-a", c.Login, "-w").Output()
		if err == nil {
			c.Token = strings.TrimSpace(string(out))
		}
	}
	return nil
}

// KeychainAvailable reports whether the macOS security tool can be used.
func KeychainAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("security")
	return err == nil
}

// SaveToken persists a new session token to the keychain or the config file.
func (c *Config) SaveToken(token string) error {
	if c.Keychain && c.Login != "" {
		c.Token = token
		if err := exec.CommandContext(context.Background(), "security", "add-generic-password", "-U", "-s", keychainService, "-a", c.Login, "-w", token).Run(); err != nil {
			return errors.New("keychain write failed: " + err.Error())
		}
		return Save(c)
	}
	c.Token = token
	return Save(c)
}

// ClearToken forgets the session everywhere.
func (c *Config) ClearToken() error {
	if c.Keychain && c.Login != "" {
		_ = exec.CommandContext(context.Background(), "security", "delete-generic-password", "-s", keychainService, "-a", c.Login).Run()
	}
	c.Token = ""
	return Save(c)
}

func Save(c *Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	out := *c
	if out.Keychain || out.TokenCmd != "" {
		out.Token = ""
	}
	b, err := yaml.Marshal(&out)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
