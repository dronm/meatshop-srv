package integration1c

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AdminResponse preserves gateway failures as diagnostic data, including an
// unhealthy gateway's 503 and authentication failures from its admin endpoints.
type AdminResponse struct {
	StatusCode int `json:"status_code"`
	Body       any `json:"body"`
}

func (c *Client) Health(ctx context.Context) (AdminResponse, error) {
	return c.adminRequest(ctx, http.MethodGet, "health", false)
}

func (c *Client) Status(ctx context.Context) (AdminResponse, error) {
	return c.adminRequest(ctx, http.MethodGet, "status", true)
}

func (c *Client) Stop(ctx context.Context) (AdminResponse, error) {
	return c.adminRequest(ctx, http.MethodPost, "stop", true)
}

func (c *Client) Start(ctx context.Context) (AdminResponse, error) {
	return c.adminRequest(ctx, http.MethodPost, "start", true)
}

func (c *Client) adminRequest(ctx context.Context, method, endpoint string, authenticated bool) (AdminResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method, buildCommandURL(c.baseURL, endpoint), nil)
	if err != nil {
		return AdminResponse{}, fmt.Errorf("create 1c admin request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if authenticated && c.auth != nil {
		req.SetBasicAuth(c.auth.UserName, c.auth.Password)
	}

	// Start/stop must run once. Following redirects could change the method or
	// forward credentials; retrying a timed-out command could repeat its effects.
	client := *c.httpClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return AdminResponse{}, fmt.Errorf("call 1c admin endpoint: %w", err)
	}
	defer resp.Body.Close()

	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return AdminResponse{}, fmt.Errorf("read 1c admin response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return AdminResponse{}, fmt.Errorf("1c admin response exceeds 1 MiB")
	}
	var payload any
	if len(body) > 0 {
		if json.Valid(body) {
			payload = json.RawMessage(body)
		} else {
			payload = string(body)
		}
	}
	return AdminResponse{StatusCode: resp.StatusCode, Body: payload}, nil
}
