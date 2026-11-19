package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const maxChatTimeFormat = "2006-01-02T15:04:05.000000Z"

type maxChatRosterCursor struct {
	CustomerName string `json:"c"`
	UserName     string `json:"u"`
	UserID       int    `json:"i"`
}

type maxChatHistoryCursor struct {
	At        string `json:"t"`
	Direction int    `json:"d"`
	ID        int64  `json:"i"`
}

func maxChatCursor(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeMaxChatCursor(raw string, value any) error {
	if len(raw) > 65536 {
		return webapp.BadRequest("Некорректный курсор чата.", nil)
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(data, value) != nil {
		return webapp.BadRequest("Некорректный курсор чата.", nil)
	}
	return nil
}

func maxChatPageSize(limit int) (int, error) {
	if limit == 0 {
		return 50, nil
	}
	if limit < 1 || limit > 200 {
		return 0, webapp.BadRequest("Размер страницы должен быть от 1 до 200.", nil)
	}
	return limit, nil
}

func chatExternalID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, webapp.BadRequest("Некорректный идентификатор MAX.", nil)
	}
	return id, nil
}

func (s *CustomerMessageService) ChatParticipants(ctx context.Context, input models.MaxChatParticipantsRequest) (*models.MaxChatParticipantPage, error) {
	if _, err := requireCustomerMessageAdmin(s.Session); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	limit, err := maxChatPageSize(input.Limit)
	if err != nil {
		return nil, err
	}
	if input.CustomerID < 0 || input.UserID < 0 || input.OrderID < 0 {
		return nil, webapp.BadRequest("Некорректный идентификатор покупателя, пользователя или заказа.", nil)
	}
	var externalID int64
	if input.MaxUserID != "" {
		externalID, err = chatExternalID(input.MaxUserID)
		if err != nil {
			return nil, err
		}
	}

	var incomingID int64
	if input.MessageID != "" {
		incomingID, err = chatExternalID(input.MessageID)
		if err != nil {
			return nil, err
		}
	}
	queryText := strings.TrimSpace(input.Query)
	handle := strings.TrimPrefix(queryText, "@")
	args := []any{queryText, input.CustomerID, input.UserID, input.OrderID, externalID, limit + 1, incomingID, handle}
	query := `
		SELECT c.id, c.name, u.id, u.max_user_id::text, u.app_username, u.username, u.is_active
		FROM public.max_users AS u
		JOIN public.customers AS c ON c.id = u.customer_id
		WHERE ($1::text = '' OR strpos(lower(c.name), lower($1)) > 0
			OR strpos(lower(u.app_username), lower($1)) > 0
			OR ($8::text <> '' AND strpos(lower(u.username), lower($8)) > 0))
			AND ($2::integer = 0 OR c.id = $2)
			AND ($3::integer = 0 OR u.id = $3)
			AND ($4::integer = 0 OR EXISTS (
				SELECT 1 FROM public.orders AS o
				WHERE o.id = $4 AND o.customer_id = c.id AND o.customer_user_id = u.id
			))
			AND ($5::bigint = 0 OR u.max_user_id = $5)
			AND ($7::bigint = 0 OR EXISTS (
				SELECT 1 FROM public.customer_messages AS m
				WHERE m.id = $7 AND m.customer_id = c.id AND m.max_user_id = u.max_user_id
			))`
	if input.Cursor != "" {
		var cursor maxChatRosterCursor
		if err := decodeMaxChatCursor(input.Cursor, &cursor); err != nil {
			return nil, err
		}
		if cursor.UserID <= 0 {
			return nil, webapp.BadRequest("Некорректный курсор списка получателей.", nil)
		}
		query += " AND (c.name, u.app_username, u.id) > ($9, $10, $11)"
		args = append(args, cursor.CustomerName, cursor.UserName, cursor.UserID)
	}
	query += " ORDER BY c.name, u.app_username, u.id LIMIT $6"
	conn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, connID)
	rows, err := conn.Conn().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list MAX chat recipients: %w", err)
	}
	defer rows.Close()
	result := &models.MaxChatParticipantPage{Rows: []models.MaxChatParticipant{}}
	for rows.Next() {
		var row models.MaxChatParticipant
		if err := rows.Scan(&row.CustomerID, &row.CustomerName, &row.UserID, &row.MaxUserID, &row.UserName, &row.Username, &row.IsActive); err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result.Rows) > limit {
		result.Rows = result.Rows[:limit]
		last := result.Rows[limit-1]
		next, err := maxChatCursor(maxChatRosterCursor{CustomerName: last.CustomerName, UserName: last.UserName, UserID: last.UserID})
		if err != nil {
			return nil, err
		}
		result.NextCursor = &next
	}
	return result, nil
}

const maxChatHistorySelect = `
	SELECT m.id, m.kind, m.message_at, m.direction_sort, m.text, m.author_name, m.status,
		m.sent_at, m.error_str, (m.direction_sort = 1 OR r.message_id IS NOT NULL)
	FROM public.max_chat_history AS m
	LEFT JOIN public.customer_message_reads AS r
		ON m.direction_sort = 0 AND r.message_id = m.id AND r.user_id = $1
	WHERE m.customer_id = $2
		AND m.max_user_id = (SELECT u.max_user_id FROM public.max_users AS u WHERE u.id = $3 AND u.customer_id = $2)`

func scanMaxChatMessage(row interface{ Scan(...any) error }) (*models.MaxChatMessage, error) {
	var message models.MaxChatMessage
	var id int64
	var at time.Time
	var direction int
	if err := row.Scan(&id, &message.Kind, &at, &direction, &message.Text, &message.AuthorName, &message.Status, &message.SentAt, &message.ErrorStr, &message.IsRead); err != nil {
		return nil, err
	}
	message.ID = strconv.FormatInt(id, 10)
	message.MessageAt = at.UTC().Format(maxChatTimeFormat)
	cursor, err := maxChatCursor(maxChatHistoryCursor{At: message.MessageAt, Direction: direction, ID: id})
	if err != nil {
		return nil, err
	}
	message.Cursor = cursor
	return &message, nil
}

func (s *CustomerMessageService) ChatHistory(ctx context.Context, input models.MaxChatHistoryRequest) (*models.MaxChatHistoryPage, error) {
	user, err := requireCustomerMessageAdmin(s.Session)
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	limit, err := maxChatPageSize(input.Limit)
	if err != nil {
		return nil, err
	}
	if input.CustomerID <= 0 || input.UserID <= 0 {
		return nil, webapp.BadRequest("Выберите покупателя и пользователя MAX.", nil)
	}
	selectors := 0
	for _, value := range []string{input.Before, input.After, input.IncomingID, input.OutgoingID} {
		if value != "" {
			selectors++
		}
	}
	if selectors > 1 {
		return nil, webapp.BadRequest("Передайте только один параметр для выбора страницы сообщений.", nil)
	}

	args := []any{user.ID, input.CustomerID, input.UserID, limit + 1}
	query := maxChatHistorySelect
	ascending := input.After != ""
	rawCursor := input.Before
	order := " ORDER BY m.message_at DESC, m.direction_sort DESC, m.id DESC"
	comparator := "<"
	if ascending {
		rawCursor = input.After
		order = " ORDER BY m.message_at, m.direction_sort, m.id"
		comparator = ">"
	}
	if rawCursor != "" {
		var cursor maxChatHistoryCursor
		if err := decodeMaxChatCursor(rawCursor, &cursor); err != nil {
			return nil, err
		}
		at, err := time.Parse(maxChatTimeFormat, cursor.At)
		if err != nil || cursor.ID <= 0 || (cursor.Direction != 0 && cursor.Direction != 1) {
			return nil, webapp.BadRequest("Некорректный курсор истории сообщений.", nil)
		}
		query += " AND (m.message_at, m.direction_sort, m.id) " + comparator + " ($5, $6, $7)"
		args = append(args, at, cursor.Direction, cursor.ID)
	}
	recordID, direction := input.OutgoingID, 1
	if input.IncomingID != "" {
		recordID, direction = input.IncomingID, 0
	}
	if recordID != "" {
		id, err := chatExternalID(recordID)
		if err != nil {
			return nil, err
		}
		query += " AND m.direction_sort = $6 AND m.id = $5"
		args = append(args, id, direction)
	}
	query += order + " LIMIT $4"

	conn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, connID)
	rows, err := conn.Conn().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read MAX chat history: %w", err)
	}
	defer rows.Close()
	result := &models.MaxChatHistoryPage{Rows: []models.MaxChatMessage{}}
	for rows.Next() {
		row, err := scanMaxChatMessage(rows)
		if err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, *row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result.Rows) > limit {
		result.HasMore = true
		result.Rows = result.Rows[:limit]
	}
	if !ascending {
		slices.Reverse(result.Rows)
	}
	return result, nil
}

var maxChatRequestID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (s *CustomerMessageService) ChatSend(ctx context.Context, input models.MaxChatSendRequest) (*models.MaxChatMessage, error) {
	admin, err := requireCustomerMessageAdmin(s.Session)
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.CustomerID <= 0 || input.UserID <= 0 || input.OrderID < 0 {
		return nil, webapp.BadRequest("Выберите покупателя и пользователя MAX.", nil)
	}
	if input.Text == "" || !utf8.ValidString(input.Text) || utf8.RuneCountInString(input.Text) > 4000 {
		return nil, webapp.BadRequest("Сообщение должно содержать от 1 до 4000 символов.", nil)
	}
	if !maxChatRequestID.MatchString(input.RequestID) {
		return nil, webapp.BadRequest("Некорректный идентификатор запроса отправки.", nil)
	}
	input.RequestID = strings.ToLower(input.RequestID)
	var result *models.MaxChatMessage
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		// Concurrent retries serialize before checking the transcript. A rolled-back
		// transaction leaves neither a queue entry nor a request ID behind.
		requestKey := fmt.Sprintf("max_chat:%d:%s", admin.ID, input.RequestID)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, requestKey); err != nil {
			return err
		}
		if input.OrderID > 0 {
			var orderID int
			err := tx.QueryRow(ctx, `
				SELECT id FROM public.orders
				WHERE id = $1 AND customer_id = $2 AND customer_user_id = $3 FOR SHARE`,
				input.OrderID, input.CustomerID, input.UserID).Scan(&orderID)
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.Conflict("Получатель заказа изменился. Обновите заказ и откройте чат ещё раз.", nil)
			}
			if err != nil {
				return err
			}
		}
		var externalID int64
		var active bool
		err := tx.QueryRow(ctx, `
			SELECT max_user_id, is_active FROM public.max_users
			WHERE id = $1 AND customer_id = $2 FOR SHARE`, input.UserID, input.CustomerID).Scan(&externalID, &active)
		if errors.Is(err, ds.ErrNoRows) {
			return webapp.NotFound("Пользователь MAX не привязан к этому покупателю.", nil)
		}
		if err != nil {
			return err
		}
		var id int64
		var same bool
		err = tx.QueryRow(ctx, `
			SELECT id, customer_id = $3 AND max_user_id = $4 AND text = $5
			FROM public.max_chat_outgoing WHERE author_user_id = $1 AND request_id = $2::uuid`,
			admin.ID, input.RequestID, input.CustomerID, externalID, input.Text).Scan(&id, &same)
		switch {
		case err == nil:
			if !same {
				return webapp.Conflict("Идентификатор отправки уже использован для другого сообщения.", nil)
			}
		case errors.Is(err, ds.ErrNoRows):
			if !active {
				return webapp.Conflict("Пользователь MAX неактивен. Отправка сообщения недоступна.", nil)
			}
			queued, err := enqueueMaxOutMessage(ctx, tx, externalID, input.Text, map[string]any{
				"source": "customer_chat", "audience": "customer",
			})
			if err != nil {
				return err
			}
			id = queued.ID
			if _, err := tx.Exec(ctx, `
				INSERT INTO public.max_chat_outgoing
					(id, customer_id, max_user_id, kind, author_user_id, author_name, request_id, text, created_at)
				VALUES ($1, $2, $3, 'manual', $4, $5, $6::uuid, $7, $8)`,
				id, input.CustomerID, externalID, admin.ID, admin.Name, input.RequestID, input.Text, queued.CreatedAt); err != nil {
				return err
			}
		default:
			return err
		}
		result, err = scanMaxChatMessage(tx.QueryRow(ctx, maxChatHistorySelect+" AND m.direction_sort = 1 AND m.id = $4",
			admin.ID, input.CustomerID, input.UserID, id))
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
