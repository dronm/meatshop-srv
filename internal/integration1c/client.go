// Package integration1c implements synchronous calls to the goCOM1c HTTP service.
package integration1c

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const (
	defaultTimeout    = 15 * time.Second
	defaultRetryDelay = 250 * time.Millisecond
)

type Config struct {
	URL        string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

type Client struct {
	executeURL string
	binDataURL string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

type commandRequest struct {
	Command string `json:"command"`
	Params  any    `json:"params"`
}

type commandResponse[T any] struct {
	Success bool   `json:"success"`
	Payload []T    `json:"payload"`
	Error   string `json:"error,omitempty"`
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("1c http error: %s", e.Status)
	}

	return fmt.Sprintf("1c http error: %s: %s", e.Status, e.Body)
}

type CommandError struct {
	Command string
	Message string
}

func (e *CommandError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("1c command %q failed", e.Command)
	}

	return fmt.Sprintf("1c command %q failed: %s", e.Command, e.Message)
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.URL)
	if baseURL == "" {
		return nil, fmt.Errorf("1c url is required")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse 1c url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("1c url should use http or https scheme")
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("1c url host is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("1c timeout should not be negative")
	}

	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		return nil, fmt.Errorf("1c max retries should not be negative")
	}

	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = defaultRetryDelay
	}
	if retryDelay < 0 {
		return nil, fmt.Errorf("1c retry delay should not be negative")
	}

	return &Client{
		executeURL: buildCommandURL(parsedURL, "execute"),
		binDataURL: buildCommandURL(parsedURL, "bin-data"),
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}, nil
}

func buildCommandURL(baseURL *url.URL, endpoint string) string {
	copyURL := *baseURL
	path := strings.TrimSuffix(copyURL.Path, "/")
	for _, suffix := range []string{"/execute", "/bin-data"} {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	copyURL.Path = path
	return copyURL.JoinPath(endpoint).String()
}

func execute[T any](ctx context.Context, client *Client, command string, params any) ([]T, error) {
	requestBody, err := marshalCommandRequest(command, params)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt <= client.maxRetries; attempt++ {
		payload, retry, err := executeAttempt[T](ctx, client, command, requestBody)
		if err == nil {
			return payload, nil
		}
		if !retry || attempt == client.maxRetries {
			return nil, err
		}
		if err := waitRetry(ctx, client.retryDelay, attempt); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("1c command %q failed", command)
}

func marshalCommandRequest(command string, params any) ([]byte, error) {
	requestBody, err := json.Marshal(commandRequest{
		Command: command,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal 1c command %q: %w", command, err)
	}
	return requestBody, nil
}

func executeAttempt[T any](ctx context.Context, client *Client, command string, requestBody []byte) ([]T, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.executeURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, false, fmt.Errorf("create 1c request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, ctxErr
		}

		return nil, isRetriableTransportError(err), fmt.Errorf("execute 1c command %q: %w", command, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read 1c command %q response: %w", command, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}

		return nil, isRetriableStatus(resp.StatusCode), httpErr
	}

	var result commandResponse[T]
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, false, fmt.Errorf("decode 1c command %q response: %w", command, err)
	}
	if !result.Success {
		return nil, false, &CommandError{
			Command: command,
			Message: result.Error,
		}
	}

	return result.Payload, false, nil
}

func isRetriableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetriableTransportError(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return !dnsErr.IsNotFound && (dnsErr.Timeout() || dnsErr.IsTemporary)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

func waitRetry(ctx context.Context, baseDelay time.Duration, attempt int) error {
	delay := baseDelay
	for i := 0; i < attempt; i++ {
		if delay >= 30*time.Second/2 {
			delay = 30 * time.Second
			break
		}
		delay *= 2
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
