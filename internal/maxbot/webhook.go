package maxbot

import (
	"crypto/hmac"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const maxWebhookBodySize = 1 << 20

type Handler struct {
	Store       *Store
	Secret      string
	BotID       int64
	welcomeBody json.RawMessage
}

func NewHandler(store *Store, secret string, botID int64) (*Handler, error) {
	welcome, err := BuildWelcomeMessage(botID)
	if err != nil {
		return nil, err
	}
	return &Handler{
		Store:       store,
		Secret:      strings.TrimSpace(secret),
		BotID:       botID,
		welcomeBody: welcome,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Secret != "" && !hmac.Equal([]byte(r.Header.Get("X-Max-Bot-Api-Secret")), []byte(h.Secret)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.Store == nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize+1))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 || len(body) > maxWebhookBodySize {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	update.UpdateType = strings.TrimSpace(update.UpdateType)
	if update.UpdateType == "" {
		http.Error(w, "update_type is required", http.StatusBadRequest)
		return
	}

	if err := h.Store.SaveUpdate(r.Context(), body, update, h.welcomeBody); err != nil {
		slog.Error("persist/process MAX update", "update_type", update.UpdateType, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func normalized(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
