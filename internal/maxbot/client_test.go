package maxbot

import (
	"encoding/json"
	"testing"
)

func TestBuildWelcomeMessage(t *testing.T) {
	body, err := BuildWelcomeMessage("https://example.test/max")
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
	if button["web_app"] != "https://example.test/max" {
		t.Fatalf("web_app = %#v", button["web_app"])
	}
}

func TestBuildWelcomeMessageWithoutExplicitURL(t *testing.T) {
	body, err := BuildWelcomeMessage("")
	if err != nil {
		t.Fatalf("BuildWelcomeMessage(): %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode welcome message: %v", err)
	}
	attachments := decoded["attachments"].([]any)
	keyboard := attachments[0].(map[string]any)
	payload := keyboard["payload"].(map[string]any)
	rows := payload["buttons"].([]any)
	buttons := rows[0].([]any)
	button := buttons[0].(map[string]any)
	if _, exists := button["web_app"]; exists {
		t.Fatalf("web_app should be omitted: %#v", button)
	}
}
