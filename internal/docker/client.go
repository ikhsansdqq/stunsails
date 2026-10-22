// Package docker talks to the Docker Engine API over its unix socket.
package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	HTTP *http.Client // no overall timeout: logs and events stream
	Base string       // "http://docker"; tests point it at httptest
}

func New(socket string) *Client {
	return &Client{
		Base: "http://docker",
		HTTP: &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
			},
			MaxIdleConns:    4,
			IdleConnTimeout: 30 * time.Second,
		}},
	}
}

// Error is a non-2xx answer from the daemon, carrying its own message.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

// IsNotFound reports a 404 from the daemon.
func IsNotFound(err error) bool {
	e, ok := err.(*Error)
	return ok && e.Status == http.StatusNotFound
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	u := c.Base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker is not reachable: %w", err)
	}
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotModified {
		defer resp.Body.Close()
		var m struct{ Message string }
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if json.Unmarshal(b, &m) != nil || m.Message == "" {
			m.Message = strings.TrimSpace(string(b))
		}
		if m.Message == "" {
			m.Message = resp.Status
		}
		return nil, &Error{Status: resp.StatusCode, Message: m.Message}
	}
	return resp, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, v any) error {
	resp, err := c.do(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

// call sends a request and decodes the reply into v when v is non-nil.
func (c *Client) call(ctx context.Context, method, path string, q url.Values, body, v any) error {
	resp, err := c.do(ctx, method, path, q, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if v == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func filters(f map[string][]string) url.Values {
	b, _ := json.Marshal(f)
	return url.Values{"filters": {string(b)}}
}

// Version is the daemon's version line, for the overview.
type Version struct {
	Version    string `json:"version"`
	APIVersion string `json:"api_version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

func (c *Client) Version(ctx context.Context) (Version, error) {
	var v struct{ Version, APIVersion, Os, Arch string }
	err := c.get(ctx, "/version", nil, &v)
	return Version{v.Version, v.APIVersion, v.Os, v.Arch}, err
}
