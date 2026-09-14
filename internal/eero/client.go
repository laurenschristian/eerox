// Package eero is a thin client for the unofficial eero cloud API (api-user.e2ro.com).
package eero

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultBase = "https://api-user.e2ro.com"
	userAgent   = "eero/3.0 (iPhone; iOS 17.0)"
)

// Client talks to the eero API with a session cookie. OnToken is called whenever
// the session token changes (login, refresh) so the caller can persist it.
type Client struct {
	Base    string
	Token   string
	OnToken func(string) error
	http    *http.Client
}

func New(base, token string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{Base: strings.TrimRight(base, "/"), Token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("eero api %d: %s", e.Code, e.Message) }

type envelope struct {
	Meta struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
	} `json:"meta"`
	Data json.RawMessage `json:"data"`
}

// ErrNotLoggedIn is returned when no session token is set.
var ErrNotLoggedIn = errors.New("not logged in: run `eerox login`")

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	raw, err := c.doRaw(ctx, method, path, body, true)
	if err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// doRaw performs one request; on a stale session it refreshes once and retries.
func (c *Client) doRaw(ctx context.Context, method, path string, body any, retry bool) (json.RawMessage, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/2.2/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Cookie", "s="+c.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
		}
		return nil, fmt.Errorf("%s %s: bad response: %w", method, path, err)
	}
	if env.Meta.Code >= 200 && env.Meta.Code < 300 {
		return env.Data, nil
	}
	if retry && env.Meta.Code == 401 && env.Meta.Error == "error.session.refresh" && c.Token != "" {
		if err := c.Refresh(ctx); err != nil {
			return nil, err
		}
		return c.doRaw(ctx, method, path, body, false)
	}
	return nil, &APIError{Code: env.Meta.Code, Message: env.Meta.Error}
}

// Raw returns the data payload of any endpoint, e.g. "networks/123/devices" or "/2.2/account".
func (c *Client) Raw(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return c.doRaw(ctx, method, path, body, true)
}

func (c *Client) setToken(t string) error {
	c.Token = t
	if c.OnToken != nil {
		return c.OnToken(t)
	}
	return nil
}

// Login starts the two-step login. The returned token is only usable after Verify.
func (c *Client) Login(ctx context.Context, identifier string) (string, error) {
	var out struct {
		UserToken string `json:"user_token"`
	}
	if err := c.do(ctx, "POST", "login", map[string]string{"login": identifier}, &out); err != nil {
		return "", err
	}
	c.Token = out.UserToken
	return out.UserToken, nil
}

// Verify submits the SMS/email code and persists the session token.
func (c *Client) Verify(ctx context.Context, code string) error {
	if err := c.do(ctx, "POST", "login/verify", map[string]string{"code": code}, nil); err != nil {
		return err
	}
	return c.setToken(c.Token)
}

func (c *Client) Refresh(ctx context.Context) error {
	raw, err := c.doRaw(ctx, "POST", "login/refresh", nil, false)
	if err != nil {
		return fmt.Errorf("session refresh failed (run `eerox login`): %w", err)
	}
	var out struct {
		UserToken string `json:"user_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.UserToken == "" {
		return errors.New("session refresh returned no token")
	}
	return c.setToken(out.UserToken)
}

func (c *Client) Logout(ctx context.Context) error {
	_, err := c.doRaw(ctx, "POST", "logout", nil, false)
	return err
}

var idRe = regexp.MustCompile(`([^/]+)$`)

// ID extracts the last path segment from an API url like /2.2/networks/123.
func ID(url string) string {
	if m := idRe.FindStringSubmatch(url); m != nil {
		return m[1]
	}
	return url
}
