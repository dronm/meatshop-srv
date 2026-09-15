package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

func (s *OrderService) Create(
	ctx context.Context,
	document *models.OrderDocument,
) (*models.OrderDetail, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateOrderDocument(document, true, 0); err != nil {
		return nil, err
	}

	var result *models.OrderDetail
	now := time.Now()
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		datePolicy, err := loadCustomerOrderDatePolicy(ctx, tx, document.CustomerID, now)
		if err != nil {
			return err
		}
		if err := datePolicy.validate(document.ForDate); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO public.orders (
				for_date,
				number_1c,
				ref_1c,
				customer_id,
				customer_sale_place_id,
				customer_user_id,
				status_id,
				comment_customer,
				comment_admin
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id, version
		`,
			document.ForDate,
			document.Number1C,
			document.Ref1C,
			document.CustomerID,
			document.CustomerSalePlaceID,
			document.CustomerUserID,
			document.StatusID,
			document.CommentCustomer,
			document.CommentAdmin,
		).Scan(&document.ID, &document.Version); err != nil {
			return err
		}

		if err := syncOrderItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildProductRegisterActions(ctx, tx, orderRecorderType, document.ID); err != nil {
			return err
		}
		if _, err := enqueueOrder1CSync(ctx, tx, document.ID, document.Version); err != nil {
			return err
		}

		result, err = fetchOrderDetail(ctx, tx, document.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("create complete order: %w", err)
	}

	return result, nil
}

func (s *OrderService) Update(
	ctx context.Context,
	input models.UpdateOrderDocumentRequest,
) (*models.OrderDetail, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if err := validateOrderDocument(input.Document, false, input.ID); err != nil {
		return nil, err
	}

	document := input.Document
	document.ID = input.ID
	var result *models.OrderDetail
	now := time.Now()
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockProductRegisterRecorders(ctx, tx, orderRecorderType, document.ID); err != nil {
			return err
		}

		var currentVersion int64
		var oldStatusID *int
		var originalCustomerUserID *int
		var originalForDate time.Time
		var originalCustomerID int
		if err := tx.QueryRow(ctx, `
			SELECT version, status_id, customer_user_id, for_date, customer_id
			FROM public.orders
			WHERE id = $1
			FOR UPDATE
		`, document.ID).Scan(
			&currentVersion,
			&oldStatusID,
			&originalCustomerUserID,
			&originalForDate,
			&originalCustomerID,
		); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("order not found", map[string]any{"id": document.ID})
			}
			return err
		}
		if currentVersion != document.Version {
			return webapp.Conflict(
				"order was changed by another request",
				map[string]any{
					"id":               document.ID,
					"expected_version": document.Version,
					"current_version":  currentVersion,
				},
			)
		}
		// Keep an existing schedule editable; apply today's policy only when its
		// customer or civil order date changes.
		if !sameOrderDate(originalForDate, document.ForDate) || originalCustomerID != document.CustomerID {
			datePolicy, err := loadCustomerOrderDatePolicy(ctx, tx, document.CustomerID, now)
			if err != nil {
				return err
			}
			if err := datePolicy.validate(document.ForDate); err != nil {
				return err
			}
		}

		oldItems, loadErr := loadOrderItemSnapshots(ctx, tx, document.ID)
		if loadErr != nil {
			return fmt.Errorf("load current order item quantities: %w", loadErr)
		}
		statusChanged := !optionalIntEqual(oldStatusID, document.StatusID)

		if err := tx.QueryRow(ctx, `
			UPDATE public.orders
			SET
				for_date = $2,
				number_1c = $3,
				ref_1c = $4,
				customer_id = $5,
				customer_sale_place_id = $6,
				customer_user_id = $7,
				status_id = $8,
				comment_customer = $9,
				comment_admin = $10,
				version = version + 1
			WHERE id = $1
			RETURNING version
		`,
			document.ID,
			document.ForDate,
			document.Number1C,
			document.Ref1C,
			document.CustomerID,
			document.CustomerSalePlaceID,
			document.CustomerUserID,
			document.StatusID,
			document.CommentCustomer,
			document.CommentAdmin,
		).Scan(&document.Version); err != nil {
			return err
		}

		if err := syncOrderItems(ctx, tx, document.ID, document.Items); err != nil {
			return err
		}
		if err := rebuildProductRegisterActions(ctx, tx, orderRecorderType, document.ID); err != nil {
			return err
		}
		if _, err := enqueueOrder1CSync(ctx, tx, document.ID, document.Version); err != nil {
			return err
		}

		var err error
		result, err = fetchOrderDetail(ctx, tx, document.ID)
		if err != nil {
			return err
		}

		changedItemIDs := changedConfirmedQuantityItemIDs(oldItems, document.Items)
		if statusChanged || len(changedItemIDs) > 0 {
			if err := enqueueOrderCustomerNotification(
				ctx,
				tx,
				originalCustomerUserID,
				orderNotificationDataFromDetail(result, statusChanged, changedItemIDs),
			); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("update complete order: %w", err)
	}

	return result, nil
}

func (s *OrderService) DocumentDetail(
	ctx context.Context,
	id int,
) (*models.OrderDetail, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("order id is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for order detail: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result, err := fetchOrderDetail(ctx, poolConn.Conn(), id)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("order not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch complete order: %w", err)
	}

	return result, nil
}

func (s *OrderService) Delete(
	ctx context.Context,
	id int,
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if id <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("order id is required", nil)
	}

	var rowsAffected int64
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := lockProductRegisterRecorders(ctx, tx, orderRecorderType, id); err != nil {
			return err
		}
		if err := removeProductRegisterActions(ctx, tx, orderRecorderType, id); err != nil {
			return err
		}

		result, err := tx.Exec(ctx, "DELETE FROM public.orders WHERE id = $1", id)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	}); err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete order: %w", err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("order not found", map[string]any{"id": id})
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}
