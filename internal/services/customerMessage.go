package services

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/pg"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const customerMessageMaxReadIDs = 500

// A defined type retains generated fields and tags while selecting from the
// request-local CTE. The current admin is always a bound server-side parameter.
type customerMessageListQuery models.CustomerMessageList

func (customerMessageListQuery) Relation() string   { return "customer_message_rows" }
func (customerMessageListQuery) CollectionAgg() any { return &wmodels.TotCount{} }

func requireCustomerMessageAdmin(sess session.Session) (models.UserLogin, error) {
	var user models.UserLogin
	if sess == nil {
		return user, apperrors.SessionRequired()
	}
	if err := sess.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}
	if user.RoleID != models.RoleIDAdmin || user.Banned {
		return models.UserLogin{}, webapp.Forbidden("Administrator access required", nil)
	}
	return user, nil
}

func (s *CustomerMessageService) List(
	ctx context.Context, params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.CustomerMessageList], error) {
	var result wmodels.CollectionResponse[*models.CustomerMessageList]
	user, err := requireCustomerMessageAdmin(s.Session)
	if err != nil {
		return result, err
	}
	if err := s.requireDB(); err != nil {
		return result, err
	}
	query, totalQuery, args, err := customerMessageCollectionSQL(user.ID, params)
	if err != nil {
		return result, err
	}
	conn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return result, fmt.Errorf("get customer message connection: %w", err)
	}
	defer s.DB.Release(conn, connID)
	return fetchCustomerMessages(ctx, conn.Conn(), query, totalQuery, args)
}

func customerMessageCollectionSQL(userID int, params modelbind.CollectionParams) (string, string, []any, error) {
	if params.From < 0 || params.Count < 0 || params.Count > 200 {
		return "", "", nil, webapp.BadRequest("from must be non-negative and count must be between 0 and 200", nil)
	}
	if params.Count == 0 {
		params.Count = 30
	}
	params.Sorter = append([]modelbind.CollectionSorter(nil), params.Sorter...)
	hasID := false
	for _, sorter := range params.Sorter {
		if sorter.Field == "id" {
			hasID = true
		}
	}
	if !hasID {
		params.Sorter = append(params.Sorter, modelbind.CollectionSorter{Field: "id", Direct: modelbind.SortParDesc})
	}
	query := pg.NewPgSelect(&customerMessageListQuery{}, &pg.PgFilters{}, &pg.PgSorters{}, &pg.PgLimit{})
	if err := modelbind.BindCollectionSelectModel(query, params); err != nil {
		return "", "", nil, err
	}
	args := []any{userID}
	dataSQL, totalSQL, err := query.BuildCollectionSQL(&args)
	if err != nil {
		return "", "", nil, err
	}
	const prefix = `WITH customer_message_rows AS (
		SELECT m.*, r.message_id IS NOT NULL AS is_read, r.read_at
		FROM public.customer_messages_list AS m
		LEFT JOIN public.customer_message_reads AS r ON r.message_id = m.id AND r.user_id = $1
	) `
	return prefix + dataSQL, prefix + totalSQL, args, nil
}

func fetchCustomerMessages(
	ctx context.Context, db ds.Querier, query, totalQuery string, args []any,
) (wmodels.CollectionResponse[*models.CustomerMessageList], error) {
	result := wmodels.CollectionResponse[*models.CustomerMessageList]{Rows: make([]*models.CustomerMessageList, 0)}
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return result, fmt.Errorf("list customer messages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		row := &models.CustomerMessageList{}
		if err := rows.Scan(&row.ID, &row.MaxMessageID, &row.MaxUserID, &row.MaxChatID,
			&row.CustomerID, &row.Customer, &row.SenderName, &row.Text,
			&row.SentAt, &row.CreatedAt, &row.IsRead, &row.ReadAt); err != nil {
			return result, fmt.Errorf("scan customer message: %w", err)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	rows.Close()
	total := &wmodels.TotCount{}
	if err := db.QueryRow(ctx, totalQuery, args...).Scan(&total.TotCount); err != nil {
		return result, fmt.Errorf("count customer messages: %w", err)
	}
	result.Agg = total
	return result, nil
}

// Detail never changes read state. The frontend acknowledges only messages
// actually displayed, using MarkRead after rendering their full text.
func (s *CustomerMessageService) Detail(ctx context.Context, id int64) (*models.CustomerMessage, error) {
	if _, err := requireCustomerMessageAdmin(s.Session); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("message id must be positive", nil)
	}
	conn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, connID)
	result := &models.CustomerMessage{}
	if err := webapp.FetchModelWithConn(ctx, conn.Conn(), models.CustomerMessageKey{ID: id}, result); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("customer message not found", nil)
		}
		return nil, fmt.Errorf("fetch customer message: %w", err)
	}
	return result, nil
}

func (s *CustomerMessageService) UnreadCount(ctx context.Context, customerID *int) (*models.CustomerMessageUnreadCount, error) {
	user, err := requireCustomerMessageAdmin(s.Session)
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if customerID != nil && *customerID <= 0 {
		return nil, webapp.BadRequest("customer_id must be positive", nil)
	}
	conn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, connID)
	result := &models.CustomerMessageUnreadCount{}
	err = conn.Conn().QueryRow(ctx, `
		SELECT count(*) FROM public.customer_messages AS m
		WHERE ($2::integer IS NULL OR m.customer_id = $2)
			AND NOT EXISTS (SELECT 1 FROM public.customer_message_reads AS r
				WHERE r.message_id = m.id AND r.user_id = $1)
	`, user.ID, customerID).Scan(&result.UnreadCount)
	if err != nil {
		return nil, fmt.Errorf("count unread customer messages: %w", err)
	}
	return result, nil
}

func validatedCustomerMessageIDs(input models.CustomerMessageMarkReadRequest) ([]int64, error) {
	if len(input.MessageIDs) == 0 || len(input.MessageIDs) > customerMessageMaxReadIDs {
		return nil, webapp.BadRequest("message_ids must contain between 1 and 500 message IDs", nil)
	}
	seen := make(map[int64]bool, len(input.MessageIDs))
	ids := make([]int64, 0, len(input.MessageIDs))
	for _, id := range input.MessageIDs {
		if id <= 0 {
			return nil, webapp.BadRequest("message IDs must be positive", nil)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *CustomerMessageService) MarkRead(
	ctx context.Context, input models.CustomerMessageMarkReadRequest,
) (wmodels.RowsAffectedResponse, error) {
	var result wmodels.RowsAffectedResponse
	user, err := requireCustomerMessageAdmin(s.Session)
	if err != nil {
		return result, err
	}
	if err := s.requireDB(); err != nil {
		return result, err
	}
	ids, err := validatedCustomerMessageIDs(input)
	if err != nil {
		return result, err
	}
	conn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return result, err
	}
	defer s.DB.Release(conn, connID)
	err = ds.WithTx(ctx, conn.Conn(), func(ctx context.Context, tx ds.Tx) error {
		var selected int
		if err := tx.QueryRow(ctx, `
			WITH selected AS (
				SELECT id FROM public.customer_messages WHERE id = ANY($2::bigint[])
			), inserted AS (
				INSERT INTO public.customer_message_reads (message_id, user_id)
				SELECT id, $1 FROM selected WHERE (SELECT count(*) FROM selected) = $3
				ON CONFLICT (message_id, user_id) DO NOTHING RETURNING message_id
			)
			SELECT (SELECT count(*) FROM selected), (SELECT count(*) FROM inserted)
		`, user.ID, ids, len(ids)).Scan(&selected, &result.RowsAffected); err != nil {
			return fmt.Errorf("mark customer messages read: %w", err)
		}
		if selected != len(ids) {
			return webapp.NotFound("one or more customer messages were not found", nil)
		}
		if result.RowsAffected > 0 {
			_, err := tx.Exec(ctx, `SELECT pg_notify('CustomerMessage.Read', $1)`,
				fmt.Sprintf(`{"user_id":%d}`, user.ID))
			return err
		}
		return nil
	})
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	return result, nil
}
