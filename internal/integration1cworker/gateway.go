package integration1cworker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Auth struct {
	UserName string
	Password string
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type GatewayClient struct {
	endpoint string
	http     *http.Client
	auth     *Auth
}

func NewGatewayClient(apiURL string, auth *Auth) (*GatewayClient, error) {
	endpoint, err := commandEndpoint(apiURL)
	if err != nil {
		return nil, err
	}

	return &GatewayClient{
		endpoint: endpoint,
		http: &http.Client{
			Timeout: 0,
		},
		auth: auth,
	}, nil
}

func NewGatewayClientWithHTTPClient(apiURL string, auth *Auth, httpClient *http.Client) (*GatewayClient, error) {
	client, err := NewGatewayClient(apiURL, auth)
	if err != nil {
		return nil, err
	}
	if httpClient != nil {
		client.http = httpClient
	}
	return client, nil
}

// Execute forwards command and params to the unchanged 1C gateway contract.
// params is intentionally kept as json.RawMessage: the worker validates no
// project-specific fields and never unmarshals the 1C response body.
func (c *GatewayClient) Execute(ctx context.Context, command string, params json.RawMessage) (Response, error) {
	if len(params) == 0 {
		params = json.RawMessage("null")
	}

	payload := struct {
		Command string          `json:"command"`
		Params  json.RawMessage `json:"params"`
	}{
		Command: command,
		Params:  params,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request envelope: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	if c.auth != nil {
		req.SetBasicAuth(c.auth.UserName, c.auth.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read HTTP response: %w", err)
	}

	return Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       responseBody,
	}, nil
}

func commandEndpoint(apiURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil {
		return "", fmt.Errorf("parse CON1C_API_URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("CON1C_API_URL must be an absolute HTTP(S) URL")
	}

	path := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(path, "/execute") && path != "execute" {
		u = u.JoinPath("execute")
	}
	return u.String(), nil
}
