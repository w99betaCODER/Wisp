// Package nodeclient is the panel-side HTTP client for a single node agent.
package nodeclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client calls one node agent's HTTP API.
type Client struct {
	base string
	http *http.Client
}

// New builds a client for the agent at addr. When tlsCfg is non-nil the client
// speaks HTTPS with mTLS; otherwise it falls back to plain HTTP (development).
func New(addr string, tlsCfg *tls.Config, timeout time.Duration) *Client {
	scheme := "http"
	tr := &http.Transport{}
	if tlsCfg != nil {
		scheme = "https"
		tr.TLSClientConfig = tlsCfg
	}
	return &Client{
		base: scheme + "://" + addr,
		http: &http.Client{Timeout: timeout, Transport: tr},
	}
}

// AddUser asks the node to add a VLESS user to its inbound.
func (c *Client) AddUser(ctx context.Context, email, uuid, flow string) error {
	body, err := json.Marshal(map[string]string{"email": email, "uuid": uuid, "flow": flow})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/users", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

// RemoveUser asks the node to drop the user with the given email.
func (c *Client) RemoveUser(ctx context.Context, email string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/v1/users/"+url.PathEscape(email), nil)
	if err != nil {
		return err
	}
	return c.do(req)
}

// Stats fetches per-user traffic deltas (email -> bytes) from the node.
func (c *Client) Stats(ctx context.Context) (map[string]int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/stats", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("node %s returned %s", req.URL.Host, resp.Status)
	}
	var out map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) do(req *http.Request) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("node %s returned %s", req.URL.Host, resp.Status)
	}
	return nil
}
