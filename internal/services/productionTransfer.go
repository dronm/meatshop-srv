package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const productionMaxOrders = 1000

func productionOrderIDs(ids []int, allowEmpty bool) ([]int, error) {
	if len(ids) > productionMaxOrders {
		return nil, webapp.BadRequest("В документе слишком много заказов.", nil)
	}
	seen := make(map[int]bool)
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, webapp.BadRequest("Некорректный номер заказа.", nil)
		}
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	if !allowEmpty && len(result) == 0 {
		return nil, webapp.BadRequest("Выберите хотя бы один заказ.", nil)
	}
	return result, nil
}

func productionDate(value string) (time.Time, error) {
	if date, err := time.Parse("2006-01-02", value); err == nil {
		return date, nil
	}
	if date, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, webapp.BadRequest("Укажите корректную дату документа и дату заявки.", nil)
}

func (s *ProductionTransferService) Prepare(ctx context.Context, input models.ProductionTransferPrepareRequest) (*models.ProductionTransferPreview, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	ids, err := productionOrderIDs(input.OrderIDs, false)
	if err != nil {
		return nil, err
	}
	conn, key, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, key)
	orders, err := readProductionOrders(ctx, conn.Conn(), ids, false)
	if err != nil {
		return nil, err
	}
	linked, err := productionLinkedOrders(ctx, conn.Conn(), ids)
	if err != nil {
		return nil, err
	}
	result := &models.ProductionTransferPreview{Orders: make([]*models.ProductionOrderRow, 0), Skipped: make([]models.ProductionTransferSkippedOrder, 0)}
	byID := make(map[int]*models.ProductionOrderRow)
	for _, order := range orders {
		byID[order.OrderID] = order
	}
	for _, id := range ids {
		order := byID[id]
		reason := productionAdditionReason(order, linked[id])
		if reason != "" {
			result.Skipped = append(result.Skipped, models.ProductionTransferSkippedOrder{OrderID: id, Reason: reason})
			continue
		}
		if result.ForDate == nil {
			date := order.ForDate
			result.ForDate = &date
		}
		if !sameProductionDate(*result.ForDate, order.ForDate) {
			return nil, webapp.BadRequest("Выберите заказы на одну дату заявки.", nil)
		}
		result.Orders = append(result.Orders, order)
	}
	return result, nil
}

func productionAdditionReason(order *models.ProductionOrderRow, linked bool) string {
	if order == nil {
		return "Заказ не найден."
	}
	if linked {
		return "Заказ уже включён в передачу в производство."
	}
	if order.StatusCode == "accepted" {
		return "Заказ уже имеет статус «Готовим»."
	}
	if order.StatusCode != "new" && order.StatusCode != "processing" {
		return "Статус заказа не допускает передачу в производство."
	}
	if order.Products == "" {
		return "В заказе нет позиций с положительным количеством."
	}
	return ""
}

func sameProductionDate(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }

func productionLinkedOrders(ctx context.Context, db ds.Querier, ids []int) (map[int]bool, error) {
	result := make(map[int]bool)
	rows, err := db.Query(ctx, `SELECT order_id FROM public.production_transfer_orders WHERE order_id=ANY($1::int[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

func readProductionOrders(ctx context.Context, db ds.Querier, ids []int, withItems bool) ([]*models.ProductionOrderRow, error) {
	result := make([]*models.ProductionOrderRow, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := db.Query(ctx, `SELECT id,version,for_date,number_1c,order_ref,customer,customer_sale_place,products,status_code,comment_customer,comment_admin
		FROM public.production_order_options WHERE id=ANY($1::int[]) ORDER BY array_position($1::int[],id)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[int]*models.ProductionOrderRow)
	for rows.Next() {
		order := &models.ProductionOrderRow{}
		if err := rows.Scan(&order.OrderID, &order.Version, &order.ForDate, &order.Number1C, &order.Order, &order.Customer, &order.CustomerSalePlace, &order.Products, &order.StatusCode, &order.CommentCustomer, &order.CommentAdmin); err != nil {
			return nil, err
		}
		order.ID = order.OrderID
		result = append(result, order)
		byID[order.OrderID] = order
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if !withItems {
		return result, nil
	}
	items, err := db.Query(ctx, `SELECT i.order_id,i.product_id,p.name,i.measure_unit_id,u.name,i.quant::text,i.use_marking
		FROM public.order_items i JOIN public.products p ON p.id=i.product_id JOIN public.measure_units u ON u.id=i.measure_unit_id
		WHERE i.order_id=ANY($1::int[]) AND i.quant>0 ORDER BY i.order_id,i.line_num,i.id`, ids)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		var id int
		var item models.ProductionOrderItem
		if err := items.Scan(&id, &item.ProductID, &item.ProductName, &item.MeasureUnitID, &item.MeasureUnitName, &item.Quant, &item.UseMarking); err != nil {
			return nil, err
		}
		if order := byID[id]; order != nil {
			order.Items = append(order.Items, item)
		}
	}
	return result, items.Err()
}

func readProductionDocument(ctx context.Context, db ds.Querier, id int, withItems bool) (*models.ProductionTransferDocument, error) {
	result := &models.ProductionTransferDocument{Orders: make([]*models.ProductionOrderRow, 0), NotificationStatus: "none"}
	if err := db.QueryRow(ctx, `SELECT id,version,document_date,for_date,comment FROM public.production_transfers WHERE id=$1`, id).Scan(&result.ID, &result.Version, &result.DocumentDate, &result.ForDate, &result.Comment); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("Передача в производство не найдена.", nil)
		}
		return nil, err
	}
	rows, err := db.Query(ctx, `SELECT id,order_id,line_num FROM public.production_transfer_orders WHERE transfer_id=$1 ORDER BY line_num,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type membership struct{ id, orderID, lineNum int }
	members := make([]membership, 0)
	ids := make([]int, 0)
	for rows.Next() {
		var row membership
		if err := rows.Scan(&row.id, &row.orderID, &row.lineNum); err != nil {
			return nil, err
		}
		members = append(members, row)
		ids = append(ids, row.orderID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	orders, err := readProductionOrders(ctx, db, ids, withItems)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]*models.ProductionOrderRow)
	for _, row := range orders {
		byID[row.OrderID] = row
	}
	for _, member := range members {
		if row := byID[member.orderID]; row != nil {
			row.ID = member.id
			row.TransferID = id
			row.LineNum = member.lineNum
			result.Orders = append(result.Orders, row)
		}
	}
	if err := db.QueryRow(ctx, `SELECT CASE WHEN count(*)=0 THEN 'none' WHEN bool_or(status='failed') THEN 'failed'
		WHEN bool_or(status='sending') THEN 'sending' WHEN bool_or(status='pending') THEN 'pending' ELSE 'sent' END,
		string_agg(DISTINCT error_str,'; ') FILTER (WHERE status='failed')
		FROM public.max_out_messages WHERE metadata->>'source'='production_transfer'
		AND metadata->>'production_transfer_id'=$1 AND metadata->>'version'=$2`, fmt.Sprint(id), fmt.Sprint(result.Version)).Scan(&result.NotificationStatus, &result.NotificationError); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ProductionTransferService) DocumentDetail(ctx context.Context, id int) (*models.ProductionTransferDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("Некорректный номер документа.", nil)
	}
	conn, key, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, key)
	return readProductionDocument(ctx, conn.Conn(), id, false)
}

func (s *ProductionTransferService) Create(ctx context.Context, input *models.ProductionTransferInput) (*models.ProductionTransferDocument, error) {
	return s.saveProduction(ctx, 0, input)
}
func (s *ProductionTransferService) Update(ctx context.Context, input models.ProductionTransferUpdateRequest) (*models.ProductionTransferDocument, error) {
	return s.saveProduction(ctx, input.ID, input.Document)
}

func (s *ProductionTransferService) saveProduction(ctx context.Context, id int, input *models.ProductionTransferInput) (*models.ProductionTransferDocument, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, webapp.BadRequest("Документ не передан.", nil)
	}
	ids, err := productionOrderIDs(input.OrderIDs, id > 0)
	if err != nil {
		return nil, err
	}
	documentDate, err := productionDate(input.DocumentDate)
	if err != nil {
		return nil, err
	}
	forDate, err := productionDate(input.ForDate)
	if err != nil {
		return nil, err
	}
	input.Comment = trimOptionalString(input.Comment)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if len(input.RequestID) > 128 {
		return nil, webapp.BadRequest("Некорректный идентификатор сохранения.", nil)
	}
	if id > 0 && input.Version <= 0 {
		return nil, webapp.BadRequest("Не указана версия документа. Обновите страницу.", nil)
	}
	var result *models.ProductionTransferDocument
	err = withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if id == 0 && input.RequestID != "" {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('production_transfer_request'),hashtext($1))`, input.RequestID); err != nil {
				return err
			}
			var existingID int
			err := tx.QueryRow(ctx, `SELECT id FROM public.production_transfers WHERE request_id=$1`, input.RequestID).Scan(&existingID)
			if err == nil {
				existing, err := readProductionDocument(ctx, tx, existingID, false)
				if err != nil {
					return err
				}
				if !productionInputEquals(existing, documentDate, forDate, input.Comment, ids) {
					return webapp.Conflict("Этот запрос сохранения уже использован для другого документа.", nil)
				}
				result = existing
				return nil
			}
			if !errors.Is(err, ds.ErrNoRows) {
				return err
			}
		}
		var previous *models.ProductionTransferDocument
		if id > 0 {
			var version int64
			if err := tx.QueryRow(ctx, `SELECT version FROM public.production_transfers WHERE id=$1 FOR UPDATE`, id).Scan(&version); err != nil {
				if errors.Is(err, ds.ErrNoRows) {
					return webapp.NotFound("Передача в производство не найдена.", nil)
				}
				return err
			}
			if version != input.Version {
				return webapp.Conflict("Документ изменён другим пользователем. Обновите страницу.", nil)
			}
			previous, err = readProductionDocument(ctx, tx, id, false)
			if err != nil {
				return err
			}
		}
		existing := make(map[int]bool)
		lockIDs := append([]int(nil), ids...)
		if previous != nil {
			for _, row := range previous.Orders {
				existing[row.OrderID] = true
				lockIDs = append(lockIDs, row.OrderID)
			}
		}
		lockIDs = productionLockIDs(lockIDs)
		if err := lockProductRegisterRecorders(ctx, tx, orderRecorderType, lockIDs...); err != nil {
			return err
		}
		for _, orderID := range lockIDs {
			var locked int
			if err := tx.QueryRow(ctx, `SELECT id FROM public.orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&locked); err != nil {
				if errors.Is(err, ds.ErrNoRows) {
					return webapp.BadRequest("Один из выбранных заказов больше не существует.", map[string]any{"order_id": orderID})
				}
				return err
			}
		}
		orders, err := readProductionOrders(ctx, tx, ids, true)
		if err != nil {
			return err
		}
		linked, err := productionLinkedOrders(ctx, tx, ids)
		if err != nil {
			return err
		}
		for _, order := range orders {
			if !sameProductionDate(order.ForDate, forDate) {
				return webapp.BadRequest("Все заказы должны соответствовать дате заявки.", map[string]any{"order_id": order.OrderID})
			}
			if !existing[order.OrderID] {
				if reason := productionAdditionReason(order, linked[order.OrderID]); reason != "" {
					return webapp.Conflict(fmt.Sprintf("Заказ №%d: %s Обновите список заказов.", order.OrderID, reason), nil)
				}
			}
			if version := input.OrderVersions[order.OrderID]; version > 0 && version != order.Version {
				return webapp.Conflict(fmt.Sprintf("Заказ №%d изменён. Обновите данные документа.", order.OrderID), nil)
			}
		}
		if previous != nil && productionInputEquals(previous, documentDate, forDate, input.Comment, ids) {
			messages, err := productionMessages(forDate, orders, previous.Version > 1)
			if err != nil {
				return err
			}
			unchanged, err := productionQueuedContentEqual(ctx, tx, id, previous.Version, messages)
			if err != nil {
				return err
			}
			if unchanged {
				result, err = readProductionDocument(ctx, tx, id, false)
				return err
			}
		}
		var version int64
		if id == 0 {
			if err := tx.QueryRow(ctx, `INSERT INTO public.production_transfers(document_date,for_date,comment,request_id)
				VALUES($1,$2,$3,NULLIF($4,'')) RETURNING id,version`, documentDate, forDate, input.Comment, input.RequestID).Scan(&id, &version); err != nil {
				return err
			}
		} else {
			if err := tx.QueryRow(ctx, `UPDATE public.production_transfers SET document_date=$2,for_date=$3,comment=$4,version=version+1 WHERE id=$1 RETURNING version`, id, documentDate, forDate, input.Comment).Scan(&version); err != nil {
				return err
			}
		}
		wanted := make(map[int]bool)
		for _, orderID := range ids {
			wanted[orderID] = true
		}
		for _, orderID := range lockIDs {
			if existing[orderID] && !wanted[orderID] {
				if _, err := tx.Exec(ctx, `DELETE FROM public.production_transfer_orders WHERE transfer_id=$1 AND order_id=$2`, id, orderID); err != nil {
					return err
				}
				if err := removeProductionOrderStatus(ctx, tx, orderID, id); err != nil {
					return err
				}
			}
		}
		for index, orderID := range ids {
			if existing[orderID] {
				if _, err := tx.Exec(ctx, `UPDATE public.production_transfer_orders SET line_num=$3 WHERE transfer_id=$1 AND order_id=$2`, id, orderID, index+1); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec(ctx, `INSERT INTO public.production_transfer_orders(transfer_id,order_id,line_num) VALUES($1,$2,$3)`, id, orderID, index+1); err != nil {
					return err
				}
				if err := addProductionOrderStatus(ctx, tx, orderID, id); err != nil {
					return err
				}
			}
		}
		messages, err := productionMessages(forDate, orders, previous != nil)
		if err != nil {
			return err
		}
		if err := enqueueProductionMessages(ctx, tx, id, version, messages); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_notify('ProductionTransfer.Update',$1)`, fmt.Sprintf(`{"result":{"id":%d}}`, id)); err != nil {
			return err
		}
		result, err = readProductionDocument(ctx, tx, id, false)
		return err
	})
	return result, err
}

func productionInputEquals(previous *models.ProductionTransferDocument, documentDate, forDate time.Time, comment *string, ids []int) bool {
	if !sameProductionDate(previous.DocumentDate, documentDate) || !sameProductionDate(previous.ForDate, forDate) || !optionalProductionStringEqual(previous.Comment, comment) || len(previous.Orders) != len(ids) {
		return false
	}
	for i, id := range ids {
		if previous.Orders[i].OrderID != id {
			return false
		}
	}
	return true
}
func optionalProductionStringEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *ProductionTransferService) Delete(ctx context.Context, id int) (wmodels.RowsAffectedResponse, error) {
	result := wmodels.RowsAffectedResponse{}
	if err := s.requireSession(); err != nil {
		return result, err
	}
	if err := s.requireDB(); err != nil {
		return result, err
	}
	err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		var locked int
		if err := tx.QueryRow(ctx, `SELECT id FROM public.production_transfers WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("Передача в производство не найдена.", nil)
			}
			return err
		}
		doc, err := readProductionDocument(ctx, tx, id, false)
		if err != nil {
			return err
		}
		ids := make([]int, 0, len(doc.Orders))
		for _, row := range doc.Orders {
			ids = append(ids, row.OrderID)
		}
		sort.Ints(ids)
		if err := lockProductRegisterRecorders(ctx, tx, orderRecorderType, ids...); err != nil {
			return err
		}
		for _, orderID := range ids {
			var locked int
			if err := tx.QueryRow(ctx, `SELECT id FROM public.orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&locked); err != nil {
				return err
			}
			if err := removeProductionOrderStatus(ctx, tx, orderID, id); err != nil {
				return err
			}
		}
		if err := cancelProductionMessages(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.production_transfers WHERE id=$1`, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `SELECT pg_notify('ProductionTransfer.Update',$1)`, fmt.Sprintf(`{"result":{"id":%d}}`, id))
		result.RowsAffected = 1
		return err
	})
	return result, err
}

func (s *ProductionTransferService) OrderOptions(ctx context.Context, input models.ProductionOrderOptionsInput) (wmodels.CollectionResponse[*models.ProductionOrderRow], error) {
	result := wmodels.CollectionResponse[*models.ProductionOrderRow]{Rows: make([]*models.ProductionOrderRow, 0)}
	if err := s.requireSession(); err != nil {
		return result, err
	}
	if err := s.requireDB(); err != nil {
		return result, err
	}
	if input.From < 0 || input.Count < 0 || input.Count > 200 {
		return result, webapp.BadRequest("Некорректные параметры списка заказов.", nil)
	}
	if input.Count == 0 {
		input.Count = 30
	}
	conn, key, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return result, err
	}
	defer s.DB.Release(conn, key)
	const where = ` WHERE ($1::int>0 AND o.id=$1) OR ($1::int=0 AND o.status_code IN ('new','processing') AND o.products<>''
		AND NOT EXISTS(SELECT 1 FROM public.production_transfer_orders l WHERE l.order_id=o.id)
		AND ($2='' OR o.id::text ILIKE $3 OR o.number_1c ILIKE $3 OR to_char(o.for_date,'DD.MM.YY') ILIKE $3 OR o.customer->>'descr' ILIKE $3))`
	var total int64
	if err := conn.Conn().QueryRow(ctx, `SELECT count(*) FROM public.production_order_options o`+where, input.OrderID, input.Search, "%"+input.Search+"%").Scan(&total); err != nil {
		return result, err
	}
	rows, err := conn.Conn().Query(ctx, `SELECT o.id FROM public.production_order_options o`+where+` ORDER BY o.id DESC OFFSET $4 LIMIT $5`, input.OrderID, input.Search, "%"+input.Search+"%", input.From, input.Count)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return result, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	rows.Close()
	result.Rows, err = readProductionOrders(ctx, conn.Conn(), ids, false)
	result.Agg = &wmodels.TotCount{TotCount: int(total)}
	return result, err
}

func (s *ProductionTransferService) ByOrder(ctx context.Context, orderID int) (*models.ProductionTransferReference, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	conn, key, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(conn, key)
	result := &models.ProductionTransferReference{}
	if err := conn.Conn().QueryRow(ctx, `SELECT transfer_id FROM public.production_transfer_orders WHERE order_id=$1`, orderID).Scan(&result.ID); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("Заказ не включён в передачу в производство.", nil)
		}
		return nil, err
	}
	return result, nil
}

func (s *ProductionTransferService) RetryMessages(ctx context.Context, id int) (wmodels.RowsAffectedResponse, error) {
	result := wmodels.RowsAffectedResponse{}
	if err := s.requireSession(); err != nil {
		return result, err
	}
	if err := s.requireDB(); err != nil {
		return result, err
	}
	if _, err := s.DocumentDetail(ctx, id); err != nil {
		return result, err
	}
	err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		count, err := retryProductionMessages(ctx, tx, id)
		result.RowsAffected = count
		return err
	})
	return result, err
}

// A document can replace its entire selection, so the lock union may contain
// twice the per-document limit. Do not run it through selection-size validation.
func productionLockIDs(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Ints(result)
	return result
}

func productionQueuedContentEqual(ctx context.Context, db ds.Querier, id int, version int64, messages []string) (bool, error) {
	rows, err := db.Query(ctx, `SELECT message->>'text' FROM public.max_out_messages
		WHERE metadata->>'source'='production_transfer' AND metadata->>'production_transfer_id'=$1
		AND metadata->>'version'=$2 ORDER BY (metadata->>'part')::integer`, fmt.Sprint(id), fmt.Sprint(version))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return false, err
		}
		if index >= len(messages) || text != messages[index] {
			return false, nil
		}
		index++
	}
	return index == len(messages), rows.Err()
}
