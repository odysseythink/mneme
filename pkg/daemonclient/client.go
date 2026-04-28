package daemonclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

// ErrUnavailable signals the daemon is not reachable.
var ErrUnavailable = errors.New("daemon unavailable")

// Client talks to the daemon over its Unix socket.
type Client struct {
	httpClient *http.Client
}

// HealthResponse mirrors GET /health.
type HealthResponse struct {
	PID       int    `json:"pid"`
	UptimeS   int64  `json:"uptime_s"`
	Version   string `json:"version"`
	StartedAt int64  `json:"started_at"`
}

// TryDial attempts to connect to the daemon at ~/.mneme/daemon/socket.
// Returns ErrUnavailable when the socket is missing or refuses connection.
func TryDial(home string) (*Client, error) {
	socketPath := state.DaemonSocketPath(home)
	if _, err := os.Stat(socketPath); err != nil {
		return nil, ErrUnavailable
	}
	conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
	if err != nil {
		return nil, ErrUnavailable
	}
	_ = conn.Close()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 200 * time.Millisecond}
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second}}, nil
}

func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var hb HealthResponse
	if err := c.do(ctx, "GET", "/health", nil, &hb); err != nil {
		return nil, err
	}
	return &hb, nil
}

func (c *Client) CronList(ctx context.Context) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if err := c.do(ctx, "GET", "/cron/list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CronRun(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/cron/run", map[string]string{"name": name}, nil)
}

func (c *Client) CronRetry(ctx context.Context, name string) error {
	return c.do(ctx, "POST", "/cron/retry", map[string]string{"name": name}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, buf)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned %d: %s", resp.StatusCode, string(data))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
