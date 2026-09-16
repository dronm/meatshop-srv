package maxbot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBuildWelcomeMessage(t *testing.T) {
	const botID int64 = 123456789

	body, err := BuildWelcomeMessage(botID)
	if err != nil {
		t.Fatalf("BuildWelcomeMessage(): %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode welcome message: %v", err)
	}
	if decoded["text"] != WelcomeMessage {
		t.Fatalf("text = %#v", decoded["text"])
	}
	attachments, ok := decoded["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v", decoded["attachments"])
	}
	keyboard := attachments[0].(map[string]any)
	payload := keyboard["payload"].(map[string]any)
	rows := payload["buttons"].([]any)
	buttons := rows[0].([]any)
	button := buttons[0].(map[string]any)
	if button["type"] != "open_app" {
		t.Fatalf("button type = %#v", button["type"])
	}
	if button["contact_id"] != float64(botID) {
		t.Fatalf("contact_id = %#v", button["contact_id"])
	}
	if _, exists := button["web_app"]; exists {
		t.Fatalf("web_app should be omitted: %#v", button)
	}
}

func TestBuildWelcomeMessageRejectsInvalidBotID(t *testing.T) {
	for _, botID := range []int64{0, -1} {
		if _, err := BuildWelcomeMessage(botID); err == nil {
			t.Fatalf("BuildWelcomeMessage(%d) should fail", botID)
		}
	}
}

func TestClientGetMe(t *testing.T) {
	client, err := NewClient("  bot-token  ")
	if err != nil {
		t.Fatalf("NewClient(): %v", err)
	}
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %q", req.Method)
		}
		if req.URL.String() != apiBaseURL+"/me" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "bot-token" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"user_id":987654321}`)),
			Request:    req,
		}, nil
	})}

	bot, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe(): %v", err)
	}
	if bot.UserID != 987654321 {
		t.Fatalf("user_id = %d", bot.UserID)
	}
}

func TestClientGetMeReturnsAPIError(t *testing.T) {
	client, err := NewClient("bot-token")
	if err != nil {
		t.Fatalf("NewClient(): %v", err)
	}
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":"verify.token"}`)),
			Request:    req,
		}, nil
	})}

	_, err = client.GetMe(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("GetMe() error = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status code = %d", apiErr.StatusCode)
	}
}

func TestClientGetMeRejectsInvalidBotID(t *testing.T) {
	client, err := NewClient("bot-token")
	if err != nil {
		t.Fatalf("NewClient(): %v", err)
	}
	client.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"user_id":0}`)),
			Request:    req,
		}, nil
	})}

	if _, err := client.GetMe(context.Background()); err == nil {
		t.Fatal("GetMe() should reject a non-positive user_id")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
