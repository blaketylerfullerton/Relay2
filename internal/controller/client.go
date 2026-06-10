package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client talks to a controller over HTTP+JSON. It is cheap to construct; the
// zero value is unusable — use NewClient so Addr and the HTTP client are set.
type Client struct {
	Addr string // host:port of the controller, e.g. "127.0.0.1:7777"
	http *http.Client
}

// NewClient returns a Client for the controller at addr. Timeouts are short so
// a wedged or unreachable controller degrades quickly rather than hanging the
// CLI, mirroring probeClient in internal/runtime.
func NewClient(addr string) *Client {
	return &Client{Addr: addr, http: &http.Client{Timeout: 3 * time.Second}}
}

// Register announces this node to the controller. It is identical on the wire
// to Heartbeat; the separate method documents intent at the call site.
func (c *Client) Register(reg Registration) error { return c.post("/v1/register", reg) }

// Heartbeat refreshes this node's liveness and current state.
func (c *Client) Heartbeat(reg Registration) error { return c.post("/v1/heartbeat", reg) }

// Cluster fetches the current fabric view from the controller.
func (c *Client) Cluster() (ClusterView, error) {
	var view ClusterView
	req, err := http.NewRequest(http.MethodGet, c.url("/v1/cluster"), nil)
	if err != nil {
		return view, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return view, fmt.Errorf("controller unreachable at %s: %w", c.Addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return view, fmt.Errorf("controller %s returned %s", c.Addr, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		return view, fmt.Errorf("decoding cluster view: %w", err)
	}
	return view, nil
}

func (c *Client) post(path string, body any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.url(path), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("controller unreachable at %s: %w", c.Addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("controller %s returned %s", c.Addr, resp.Status)
	}
	return nil
}

func (c *Client) url(path string) string { return "http://" + c.Addr + path }
