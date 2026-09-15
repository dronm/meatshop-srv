package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const orderDocumentMaxItems = 1000

func validateOrderDocument(document *models.OrderDocument, create bool, pathID int) error {
	if document == nil {
		return webapp.BadRequest("order document is required", nil)
	}
	if err := validateOrderDocumentIdentity(document.ID, document.Version, create, pathID); err != nil {
		return err
	}
	if document.ForDate.IsZero() {
		return webapp.BadRequest("order for_date is required", nil)
	}
	if document.CustomerID <= 0 {
		return webapp.BadRequest("order customer_id should be positive", nil)
	}
	if document.CustomerSalePlaceID <= 0 {
		return webapp.BadRequest("order customer_sale_place_id should be positive", nil)
	}
	if document.CustomerUserID != nil && *document.CustomerUserID <= 0 {
		return webapp.BadRequest("order customer_user_id should be positive", nil)
	}
	if document.StatusID != nil && *document.StatusID <= 0 {
		return webapp.BadRequest("order status_id should be positive", nil)
	}

	document.Number1C = trimOptionalString(document.Number1C)
	document.CommentCustomer = trimOptionalString(document.CommentCustomer)
	document.CommentAdmin = trimOptionalString(document.CommentAdmin)
	if document.Ref1C != nil {
		document.Ref1C.ID = strings.TrimSpace(document.Ref1C.ID)
		document.Ref1C.Descr = strings.TrimSpace(document.Ref1C.Descr)
		if document.Ref1C.ID == "" {
			document.Ref1C = nil
		}
	}

	if len(document.Items) == 0 {
		return webapp.BadRequest("order should contain at least one item", nil)
	}
	if len(document.Items) > orderDocumentMaxItems {
		return webapp.BadRequest(
			"order contains too many items",
			map[string]any{"maximum": orderDocumentMaxItems, "count": len(document.Items)},
		)
	}

	seenIDs := make(map[int]struct{}, len(document.Items))
	for index, item := range document.Items {
		if item == nil {
			return invalidOrderDocumentItem(index, "item is required")
		}
		if item.ID < 0 {
			return invalidOrderDocumentItem(index, "id should not be negative")
		}
		if create && item.ID != 0 {
			return invalidOrderDocumentItem(index, "a new order item should not contain id")
		}
		if item.ID != 0 {
			if _, exists := seenIDs[item.ID]; exists {
				return invalidOrderDocumentItem(index, "id is duplicated")
			}
			seenIDs[item.ID] = struct{}{}
		}

		item.LineNum = index + 1
		if item.ProductID <= 0 {
			return invalidOrderDocumentItem(index, "product_id should be positive")
		}
		if item.MeasureUnitID <= 0 {
			return invalidOrderDocumentItem(index, "measure_unit_id should be positive")
		}
		if item.QuantRequired <= 0 {
			return invalidOrderDocumentItem(index, "quant_required should be greater than zero")
		}
	}

	return nil
}

func validateOrderDocumentIdentity(bodyID int, version int64, create bool, pathID int) error {
	if create {
		if bodyID != 0 {
			return webapp.BadRequest("a new order should not contain id", map[string]any{"id": bodyID})
		}
		if version != 0 {
			return webapp.BadRequest("a new order should not contain version", map[string]any{"version": version})
		}
		return nil
	}

	if pathID <= 0 {
		return webapp.BadRequest("order id should be positive", nil)
	}
	if bodyID != 0 && bodyID != pathID {
		return webapp.BadRequest(
			"order body id does not match path id",
			map[string]any{"path_id": pathID, "body_id": bodyID},
		)
	}
	if version <= 0 {
		return webapp.BadRequest("order version should be positive", nil)
	}

	return nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func invalidOrderDocumentItem(index int, message string) error {
	return webapp.BadRequest(
		fmt.Sprintf("items[%d]: %s", index, message),
		map[string]any{"item_index": index},
	)
}

func fetchOrderDetail(
	ctx context.Context,
	db ds.Querier,
	id int,
) (*models.OrderDetail, error) {
	rows, err := db.Query(ctx, `
		SELECT
			orders.id,
			orders.version,
			orders.for_date,
			orders.number_1c,
			orders.ref_1c,
			orders.customer_id,
			customers_ref(customer) AS customer,
			orders.customer_sale_place_id,
			customer_sale_places_ref(sale_place) AS customer_sale_place,
			orders.customer_user_id,
			customer_users_ref(customer_user) AS customer_user,
			orders.status_id,
			order_statuses_ref(order_status) AS status,
			orders.comment_customer,
			orders.comment_admin,
			item.id,
			item.line_num,
			item.product_id,
			products_ref(product) AS product,
			item.measure_unit_id,
			measure_units_ref(measure_unit) AS measure_unit,
			item.quant_required::double precision,
			item.quant::double precision
		FROM public.orders AS orders
		JOIN public.customers AS customer
			ON customer.id = orders.customer_id
		JOIN public.customer_sale_places AS sale_place
			ON sale_place.id = orders.customer_sale_place_id
		LEFT JOIN public.max_users AS customer_user
			ON customer_user.id = orders.customer_user_id
		LEFT JOIN public.order_statuses AS order_status
			ON order_status.id = orders.status_id
		LEFT JOIN public.order_items AS item
			ON item.order_id = orders.id
		LEFT JOIN public.products AS product
			ON product.id = item.product_id
		LEFT JOIN public.measure_units AS measure_unit
			ON measure_unit.id = item.measure_unit_id
		WHERE orders.id = $1
		ORDER BY item.line_num, item.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var detail *models.OrderDetail
	for rows.Next() {
		var documentID, customerID, customerSalePlaceID int
		var version int64
		var forDate time.Time
		var number1C, commentCustomer, commentAdmin *string
		var ref1C *models.Ref1c
		var customerUserID, statusID *int
		var customer, customerSalePlace, customerUser, status *models.Ref
		var itemID, lineNum, productID, measureUnitID *int
		var product, measureUnit *models.Ref
		var quantRequired, quant *float64

		if err := rows.Scan(
			&documentID,
			&version,
			&forDate,
			&number1C,
			&ref1C,
			&customerID,
			&customer,
			&customerSalePlaceID,
			&customerSalePlace,
			&customerUserID,
			&customerUser,
			&statusID,
			&status,
			&commentCustomer,
			&commentAdmin,
			&itemID,
			&lineNum,
			&productID,
			&product,
			&measureUnitID,
			&measureUnit,
			&quantRequired,
			&quant,
		); err != nil {
			return nil, err
		}

		if detail == nil {
			detail = &models.OrderDetail{
				ID:                  documentID,
				Version:             version,
				ForDate:             forDate,
				Number1C:            number1C,
				Ref1C:               ref1C,
				CustomerID:          customerID,
				Customer:            customer,
				CustomerSalePlaceID: customerSalePlaceID,
				CustomerSalePlace:   customerSalePlace,
				CustomerUserID:      customerUserID,
				CustomerUser:        customerUser,
				StatusID:            statusID,
				Status:              status,
				CommentCustomer:     commentCustomer,
				CommentAdmin:        commentAdmin,
				Items:               make([]*models.OrderDetailItem, 0),
			}
		}

		if itemID != nil {
			detail.Items = append(detail.Items, &models.OrderDetailItem{
				ID:            *itemID,
				LineNum:       *lineNum,
				ProductID:     *productID,
				Product:       product,
				MeasureUnitID: *measureUnitID,
				MeasureUnit:   measureUnit,
				QuantRequired: *quantRequired,
				Quant:         *quant,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, ds.ErrNoRows
	}

	return detail, nil
}

func syncOrderItems(
	ctx context.Context,
	tx ds.Querier,
	orderID int,
	items []*models.OrderDocumentItem,
) error {
	existing, err := orderDocumentItemIDs(ctx, tx, orderID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		WITH line_offset AS (
			SELECT COALESCE(MAX(line_num), 0) + $2 AS value
			FROM public.order_items
			WHERE order_id = $1
		)
		UPDATE public.order_items AS item
		SET line_num = item.line_num + line_offset.value
		FROM line_offset
		WHERE item.order_id = $1
	`, orderID, orderDocumentMaxItems+1); err != nil {
		return err
	}

	for index, item := range items {
		if item.ID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO public.order_items (
					line_num,
					order_id,
					product_id,
					measure_unit_id,
					quant_required,
					quant
				)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id
			`, index+1, orderID, item.ProductID, item.MeasureUnitID, item.QuantRequired, item.Quant).Scan(&item.ID); err != nil {
				return err
			}
			continue
		}

		if _, ok := existing[item.ID]; !ok {
			return invalidOrderDocumentItem(index, "id does not belong to this order")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE public.order_items
			SET
				line_num = $3,
				product_id = $4,
				measure_unit_id = $5,
				quant_required = $6,
				quant = $7
			WHERE id = $1
				AND order_id = $2
		`, item.ID, orderID, index+1, item.ProductID, item.MeasureUnitID, item.QuantRequired, item.Quant); err != nil {
			return err
		}
		delete(existing, item.ID)
	}

	for itemID := range existing {
		if _, err := tx.Exec(
			ctx,
			"DELETE FROM public.order_items WHERE id = $1 AND order_id = $2",
			itemID,
			orderID,
		); err != nil {
			return err
		}
	}

	return nil
}

func orderDocumentItemIDs(
	ctx context.Context,
	tx ds.Querier,
	orderID int,
) (map[int]struct{}, error) {
	rows, err := tx.Query(ctx, `
		SELECT id
		FROM public.order_items
		WHERE order_id = $1
		FOR UPDATE
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
