package maxbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBaseURL = "https://platform-api2.max.ru"

const (
	WelcomeMessage = "Для оформления заказа откройте приложение."
	OpenAppText    = "Открыть приложение"
)

type Client struct {
	token string
	http  *http.Client
}

type BotInfo struct {
	UserID int64 `json:"user_id"`
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("MAX API HTTP %d: %s", e.StatusCode, e.Body)
}

func NewClient(token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("MAX bot token is required")
	}
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *Client) GetMe(ctx context.Context) (BotInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/me", nil)
	if err != nil {
		return BotInfo{}, err
	}
	req.Header.Set("Authorization", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return BotInfo{}, fmt.Errorf("get MAX bot info: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return BotInfo{}, fmt.Errorf("read MAX bot info response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return BotInfo{}, &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(responseBody)),
		}
	}

	var bot BotInfo
	if err := json.Unmarshal(responseBody, &bot); err != nil {
		return BotInfo{}, fmt.Errorf("decode MAX bot info response: %w", err)
	}
	if bot.UserID <= 0 {
		return BotInfo{}, fmt.Errorf("MAX bot info response does not contain a valid user_id")
	}
	return bot, nil
}

func (c *Client) ConfigureWebhook(ctx context.Context, webhookURL, secret string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil
	}
	body := map[string]any{"url": webhookURL}
	if secret = strings.TrimSpace(secret); secret != "" {
		body["secret"] = secret
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode MAX webhook subscription: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/subscriptions", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("configure MAX webhook: %w", err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return fmt.Errorf("decode MAX webhook subscription response: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("configure MAX webhook: %s", strings.TrimSpace(result.Message))
		}
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, userID int64, body json.RawMessage) error {
	if userID <= 0 {
		return fmt.Errorf("MAX user id should be positive")
	}
	if len(body) == 0 || !json.Valid(body) {
		return fmt.Errorf("MAX message body should be valid JSON")
	}

	endpoint := apiBaseURL + "/messages?user_id=" + url.QueryEscape(fmt.Sprintf("%d", userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send MAX message: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(responseBody)),
		}
	}
	return nil
}

func BuildWelcomeMessage(botID int64) (json.RawMessage, error) {
	if botID <= 0 {
		return nil, fmt.Errorf("MAX bot id should be positive")
	}
	button := map[string]any{
		"type":       "open_app",
		"text":       OpenAppText,
		"contact_id": botID,
	}

	body := map[string]any{
		"text": WelcomeMessage,
		"attachments": []any{
			map[string]any{
				"type": "inline_keyboard",
				"payload": map[string]any{
					"buttons": [][]any{{button}},
				},
			},
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode MAX welcome message: %w", err)
	}
	return encoded, nil
}
