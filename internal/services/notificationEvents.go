package services

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

const (
	notificationEventOrderSubmitted = "order.submitted"
	notificationEventOrderChanged   = "order.changed"
)

type orderNotificationData struct {
	OrderID         int
	Number1C        string
	ForDate         string
	CustomerName    string
	SalePlaceName   string
	StatusName      string
	CommentCustomer string
	StatusChanged   bool
	ChangedItems    string
	Resubmitted     bool
}

type orderItemSnapshot struct {
	QuantRequired float64
	Quant         float64
}

// enqueueOrderResponsibleNotifications sends one rendered notification for each
// configured responsible internal user whose users.max_user_id points to an
// active MAX account.
func enqueueOrderResponsibleNotifications(
	ctx context.Context,
	tx ds.Querier,
	event string,
	data orderNotificationData,
) error {
	rows, err := tx.Query(ctx, `
		SELECT
			t.id,
			t.code,
			t.body_template,
			mu.max_user_id
		FROM public.notification_templates AS t
		JOIN public.notification_template_recipients AS r
			ON r.template_id = t.id
		JOIN public.users AS u
			ON u.id = r.user_id
		JOIN public.max_users AS mu
			ON mu.id = u.max_user_id
			AND mu.is_active
		WHERE t.event = $1
			AND t.is_active
		ORDER BY t.id, r.id
	`, event)
	if err != nil {
		return fmt.Errorf("fetch notification recipients for %s: %w", event, err)
	}

	type target struct {
		templateID int
		code       string
		body       string
		maxUserID  int64
	}
	targets := make([]target, 0)
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.templateID, &item.code, &item.body, &item.maxUserID); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	// Do not issue INSERTs while the SELECT rows are still open: pgx keeps the
	// transaction connection busy until the result set is closed.
	rendered := make(map[int]string)
	for _, item := range targets {
		text, ok := rendered[item.templateID]
		if !ok {
			text, err = renderNotificationTemplate(item.code, item.body, data)
			if err != nil {
				return err
			}
			rendered[item.templateID] = text
		}

		if _, err := enqueueMaxOutMessage(ctx, tx, item.maxUserID, text, map[string]any{
			"event":                    event,
			"entity_type":              "order",
			"entity_id":                data.OrderID,
			"notification_template_id": item.templateID,
			"notification_code":        item.code,
		}); err != nil {
			return fmt.Errorf("enqueue responsible-user MAX notification: %w", err)
		}
	}
	return nil
}

// enqueueOrderCustomerNotification sends the order.changed event to the MAX
// account that originally submitted the order. customerMaxUserID is max_users.id,
// not the external MAX platform identifier.
func enqueueOrderCustomerNotification(
	ctx context.Context,
	tx ds.Querier,
	customerMaxUserID *int,
	data orderNotificationData,
) error {
	if customerMaxUserID == nil || *customerMaxUserID <= 0 {
		return nil
	}

	rows, err := tx.Query(ctx, `
		SELECT
			t.id,
			t.code,
			t.body_template,
			mu.max_user_id
		FROM public.notification_templates AS t
		JOIN public.max_users AS mu
			ON mu.id = $2
			AND mu.is_active
		WHERE t.event = $1
			AND t.is_active
		ORDER BY t.id
	`, notificationEventOrderChanged, *customerMaxUserID)
	if err != nil {
		return fmt.Errorf("fetch customer notification template: %w", err)
	}

	type target struct {
		templateID int
		code       string
		body       string
		maxUserID  int64
	}
	targets := make([]target, 0)
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.templateID, &item.code, &item.body, &item.maxUserID); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range targets {
		text, err := renderNotificationTemplate(item.code, item.body, data)
		if err != nil {
			return err
		}
		if _, err := enqueueMaxOutMessage(ctx, tx, item.maxUserID, text, map[string]any{
			"event":                    notificationEventOrderChanged,
			"entity_type":              "order",
			"entity_id":                data.OrderID,
			"notification_template_id": item.templateID,
			"notification_code":        item.code,
		}); err != nil {
			return fmt.Errorf("enqueue customer MAX notification: %w", err)
		}
	}
	return nil
}

func renderNotificationTemplate(code, body string, data orderNotificationData) (string, error) {
	tpl, err := template.New(code).Option("missingkey=error").Parse(body)
	if err != nil {
		return "", fmt.Errorf("parse notification template %q: %w", code, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render notification template %q: %w", code, err)
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("notification template %q rendered an empty message", code)
	}
	return text, nil
}

func orderNotificationDataFromMaxDetail(detail *models.MaxOrderDetail, resubmitted bool) orderNotificationData {
	if detail == nil {
		return orderNotificationData{}
	}
	result := orderNotificationData{
		OrderID:       detail.ID,
		ForDate:       formatNotificationDate(detail.ForDate),
		CustomerName:  detail.Customer.Descr,
		SalePlaceName: detail.CustomerSalePlace.Descr,
		StatusName:    detail.StatusName,
		Resubmitted:   resubmitted,
	}
	if detail.Number1C != nil {
		result.Number1C = strings.TrimSpace(*detail.Number1C)
	}
	if detail.CommentCustomer != nil {
		result.CommentCustomer = strings.TrimSpace(*detail.CommentCustomer)
	}
	return result
}

func orderNotificationDataFromDetail(
	detail *models.OrderDetail,
	statusChanged bool,
	changedItemIDs map[int]struct{},
) orderNotificationData {
	if detail == nil {
		return orderNotificationData{}
	}
	result := orderNotificationData{
		OrderID:       detail.ID,
		ForDate:       formatNotificationDate(detail.ForDate),
		StatusChanged: statusChanged,
	}
	if detail.Number1C != nil {
		result.Number1C = strings.TrimSpace(*detail.Number1C)
	}
	if detail.Customer != nil {
		result.CustomerName = detail.Customer.Descr
	}
	if detail.CustomerSalePlace != nil {
		result.SalePlaceName = detail.CustomerSalePlace.Descr
	}
	if detail.Status != nil {
		result.StatusName = detail.Status.Descr
	}
	if detail.CommentCustomer != nil {
		result.CommentCustomer = strings.TrimSpace(*detail.CommentCustomer)
	}

	if len(changedItemIDs) > 0 {
		lines := make([]string, 0, len(changedItemIDs))
		for _, item := range detail.Items {
			if item == nil {
				continue
			}
			if _, ok := changedItemIDs[item.ID]; !ok {
				continue
			}
			name := strconv.Itoa(item.ProductID)
			if item.Product != nil && strings.TrimSpace(item.Product.Descr) != "" {
				name = item.Product.Descr
			}
			lines = append(lines, fmt.Sprintf(
				"• %s: %s → %s",
				name,
				formatNotificationQuantity(item.QuantRequired),
				formatNotificationQuantity(item.Quant),
			))
		}
		result.ChangedItems = strings.Join(lines, "\n")
	}
	return result
}

func loadOrderItemSnapshots(ctx context.Context, tx ds.Querier, orderID int) (map[int]orderItemSnapshot, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, quant_required::double precision, quant::double precision
		FROM public.order_items
		WHERE order_id = $1
		FOR UPDATE
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]orderItemSnapshot)
	for rows.Next() {
		var id int
		var snapshot orderItemSnapshot
		if err := rows.Scan(&id, &snapshot.QuantRequired, &snapshot.Quant); err != nil {
			return nil, err
		}
		result[id] = snapshot
	}
	return result, rows.Err()
}

func changedConfirmedQuantityItemIDs(
	oldItems map[int]orderItemSnapshot,
	newItems []*models.OrderDocumentItem,
) map[int]struct{} {
	result := make(map[int]struct{})
	for _, item := range newItems {
		if item == nil || item.Quant == item.QuantRequired {
			continue
		}
		old, existed := oldItems[item.ID]
		if !existed || old.Quant != item.Quant || old.QuantRequired != item.QuantRequired {
			result[item.ID] = struct{}{}
		}
	}
	return result
}

func optionalIntEqual(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func formatNotificationDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("02.01.2006")
}

func formatNotificationQuantity(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
