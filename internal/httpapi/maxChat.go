package httpapi

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func maxChatRoutes(api *webapp.Group) {
	api.GET("/max-chat/participants",
		webapp.WithName("customerMessage.chatParticipants"),
		webapp.WithPermission("customerMessage.list"),
		webapp.WithService("CustomerMessage", "ChatParticipants"),
		webapp.WithBinder(maxChatParticipantsBinder),
	)
	api.GET("/max-chat/messages",
		webapp.WithName("customerMessage.chatHistory"),
		webapp.WithPermission("customerMessage.list"),
		webapp.WithService("CustomerMessage", "ChatHistory"),
		webapp.WithBinder(maxChatHistoryBinder),
	)
	api.POST("/max-chat/messages",
		webapp.WithName("customerMessage.chatSend"),
		webapp.WithPermission("customerMessage.send"),
		webapp.WithService("CustomerMessage", "ChatSend"),
		webapp.WithBinder(maxJSONBinder[models.MaxChatSendRequest]()),
	)
}

func maxChatQueryIDs(values url.Values, fields map[string]*int) error {
	for name, dest := range fields {
		raw := values.Get(name)
		if raw == "" {
			continue
		}
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return webapp.BadRequest("Некорректное значение параметра "+name+".", nil)
		}
		*dest = id
	}
	return nil
}

func maxChatParticipantsBinder(r *http.Request) (any, error) {
	params := r.URL.Query()
	input := models.MaxChatParticipantsRequest{
		Query: params.Get("q"), Cursor: params.Get("cursor"), MaxUserID: params.Get("max_user_id"), MessageID: params.Get("message_id"),
	}
	err := maxChatQueryIDs(params, map[string]*int{
		"customer_id": &input.CustomerID, "user_id": &input.UserID, "order_id": &input.OrderID, "limit": &input.Limit,
	})
	return input, err
}

func maxChatHistoryBinder(r *http.Request) (any, error) {
	params := r.URL.Query()
	input := models.MaxChatHistoryRequest{
		Before: params.Get("before"), After: params.Get("after"), OutgoingID: params.Get("outgoing_id"), IncomingID: params.Get("incoming_id"),
	}
	err := maxChatQueryIDs(params, map[string]*int{
		"customer_id": &input.CustomerID, "user_id": &input.UserID, "limit": &input.Limit,
	})
	return input, err
}
