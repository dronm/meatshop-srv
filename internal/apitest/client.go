//go:build integration

package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
)

type Client struct {
	baseURL string
	http    *http.Client
	user    string
	pwd     string
}

func NewClient(t *testing.T) *Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}

	baseURL := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:59000"
	}

	user := os.Getenv("API_USER")
	if user == "" {
		user = "admin"
	}
	pwd := os.Getenv("API_PWD")
	if pwd == "" {
		pwd = "123456"
	}

	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Jar: jar,
		},
		user: user,
		pwd:  pwd,
	}
}

func (c *Client) Login(t *testing.T) {
	t.Helper()

	c.DoJSON(
		t,
		http.MethodPost,
		"/api/users/login",
		map[string]any{
			"name": c.user,
			"pwd":  c.pwd,
		},
		http.StatusOK,
	)
}

func (c *Client) DoJSON(
	t *testing.T,
	method string,
	path string,
	body any,
	expectedStatus int,
) map[string]any {
	t.Helper()

	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal %s %s body: %v", method, path, err)
		}
		requestBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, requestBody)
	if err != nil {
		t.Fatalf("create %s %s request: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, path, err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf(
			"%s %s: expected HTTP %d, got %d\nbody: %s",
			method,
			path,
			expectedStatus,
			resp.StatusCode,
			strings.TrimSpace(string(data)),
		)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var result []any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("decode %s %s response as JSON array: %v\nbody: %s", method, path, err, string(data))
		}
		return map[string]any{
			"_array": result,
		}
	}

	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode %s %s response as JSON object: %v\nbody: %s", method, path, err, string(data))
	}

	return result
}

func (c *Client) DeleteIgnore(t *testing.T, path string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		t.Fatalf("create DELETE %s request: %v", path, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("DELETE %s: unexpected HTTP %d\nbody: %s", path, resp.StatusCode, string(data))
	}
}

func requireObjectField(t *testing.T, object map[string]any, field string) any {
	t.Helper()

	value, ok := object[field]
	if !ok {
		t.Fatalf("response field %q is missing: %s", field, fmt.Sprint(object))
	}
	return value
}
