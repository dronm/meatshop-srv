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

func TestSendToChatUsesExactSignedIDAndReturnsMessageID(t *testing.T) {
	client, err := NewClient("token")
	if err != nil {
		t.Fatal(err)
	}
	chatID := int64(-9223372036854775807)
	client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("chat_id") != "-9223372036854775807" || request.URL.Query().Has("user_id") {
			t.Fatalf("incorrect chat destination: %s", request.URL)
		}
		if request.Header.Get("Authorization") != "token" || request.Method != http.MethodPost {
			t.Fatalf("incorrect authorization or method: %#v", request)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"message":{"body":{"mid":"mid.123"}}}`))}, nil
	})}
	mid, err := client.SendToDestination(context.Background(), nil, &chatID, json.RawMessage(`{"text":"Заявка"}`))
	if err != nil || mid != "mid.123" {
		t.Fatalf("message id = %q, error = %v", mid, err)
	}
}

func TestSendMessageRetainsUserDestination(t *testing.T) {
	client, _ := NewClient("token")
	client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("user_id") != "77" || request.URL.Query().Has("chat_id") {
			t.Fatalf("incorrect user destination: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	if err := client.SendMessage(context.Background(), 77, json.RawMessage(`{"text":"Уведомление"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestSendToDestinationRejectsInvalidDestinationBeforeHTTP(t *testing.T) {
	client, _ := NewClient("token")
	client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatal("invalid destination reached HTTP transport")
		return nil, nil
	})}
	valid, zero, negative := int64(77), int64(0), int64(-1)
	for _, destination := range []struct{ user, chat *int64 }{
		{}, {&valid, &valid}, {&zero, nil}, {&negative, nil}, {nil, &zero},
	} {
		if _, err := client.SendToDestination(context.Background(), destination.user, destination.chat, json.RawMessage(`{"text":"test"}`)); err == nil {
			t.Fatalf("invalid destination was accepted: %#v", destination)
		}
	}
}

func TestSendTextLimitCountsUnicodeCharacters(t *testing.T) {
	client, _ := NewClient("token")
	requests := 0
	client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	chatID := int64(-7)
	for _, count := range []int{4000, 4001} {
		body, _ := json.Marshal(map[string]string{"text": strings.Repeat("Я", count)})
		_, err := client.SendToDestination(context.Background(), nil, &chatID, body)
		if (err == nil) != (count == 4000) {
			t.Fatalf("count %d: %v", count, err)
		}
	}
	if requests != 1 {
		t.Fatalf("HTTP requests = %d, want only the 4000-character message", requests)
	}
}

func TestSendToChatRecognizesMAXFailureInsideSuccessfulHTTPResponse(t *testing.T) {
	client, _ := NewClient("token")
	client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"success":false,"code":"chat.denied","message":"Denied"}`))}, nil
	})}
	chatID := int64(-7)
	_, err := client.SendToDestination(context.Background(), nil, &chatID, json.RawMessage(`{"text":"test"}`))
	var apiError *APIError
	if !errors.As(err, &apiError) || isRetryableSendError(err) {
		t.Fatalf("failure should be a non-retryable API error, got %v", err)
	}
}

func TestBotAddedUpdatePreservesSignedChatID(t *testing.T) {
	var update Update
	if err := json.Unmarshal([]byte(`{"update_type":"bot_added","chat_id":-9223372036854775808,"user":{"user_id":77}}`), &update); err != nil {
		t.Fatal(err)
	}
	if got := updateMaxChatID(update); got != int64(-9223372036854775808) {
		t.Fatalf("chat id = %#v, want an exact signed int64 value", got)
	}
}
