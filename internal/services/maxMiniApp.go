package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/config"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
)

const (
	maxSessionUserKey             = "max_user_id"
	maxRegistrationCustomerKey    = "max_registration_customer_id"
	maxRegistrationAppUsernameKey = "max_registration_app_username"
	defaultMaxAppUsername         = "Не задано"
)

const orderListCount = 10

type MaxMiniAppService struct {
	DB         ds.Provider
	Session    session.Session
	BotToken   string
	InitMaxAge time.Duration
}

type maxInitUser struct {
	ID           int64   `json:"id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Username     *string `json:"username"`
	LanguageCode string  `json:"language_code"`
	PhotoURL     *string `json:"photo_url"`
}

func NewMaxMiniAppService(cfg config.MAXConfig) func(webapp.ServiceContext) any {
	return func(ctx webapp.ServiceContext) any {
		maxAge, _ := cfg.InitDataMaxAgeDuration()
		return &MaxMiniAppService{
			DB:         ctx.DB,
			Session:    ctx.Session,
			BotToken:   strings.TrimSpace(cfg.BotToken),
			InitMaxAge: maxAge,
		}
	}
}

func RegisterMaxMiniAppService(cfg config.MAXConfig) {
	webapp.MustRegisterService(
		"MaxMiniApp",
		&MaxMiniAppService{},
		NewMaxMiniAppService(cfg),
	)
}

func (s *MaxMiniAppService) SessionStart(ctx context.Context, input models.MaxSessionRequest) (*models.MaxSessionResponse, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	if s.DB == nil {
		return nil, webapp.Internal("database is not initialized", nil)
	}
	if s.BotToken == "" {
		return nil, webapp.Internal("MAX bot token is not configured", nil)
	}

	user, rawUser, err := validateMaxInitData(strings.TrimSpace(input.InitData), s.BotToken, s.InitMaxAge, time.Now())
	if err != nil {
		return nil, webapp.Forbidden("invalid MAX init data", map[string]any{"error": err.Error()})
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for MAX session: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	var id int
	var username *string
	var appUsername string
	var customerID, salePlaceID *int
	initialUsername, initialAppUsername := initialMaxUsernames(user.Username)
	if err := poolConn.Conn().QueryRow(ctx, `
		INSERT INTO public.max_users (
			max_user_id,
			username,
			app_username,
			avatar_url,
			raw_user,
			is_active
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, true)
		ON CONFLICT (max_user_id) DO UPDATE
		SET
			username = EXCLUDED.username,
			avatar_url = EXCLUDED.avatar_url,
			raw_user = EXCLUDED.raw_user,
			is_active = true
		RETURNING id, username, app_username, customer_id, customer_sale_place_id
	`, user.ID, initialUsername, initialAppUsername, normalizedOptionalString(user.PhotoURL), string(rawUser)).Scan(
		&id,
		&username,
		&appUsername,
		&customerID,
		&salePlaceID,
	); err != nil {
		return nil, fmt.Errorf("upsert MAX user: %w", err)
	}

	if err := s.Session.Set(maxSessionUserKey, id); err != nil {
		return nil, fmt.Errorf("set MAX session identity: %w", err)
	}
	if err := s.Session.Flush(); err != nil {
		return nil, fmt.Errorf("flush MAX session identity: %w", err)
	}

	return s.sessionResponse(ctx, id, user.ID, username, appUsername, customerID, salePlaceID)
}

func (s *MaxMiniAppService) Me(ctx context.Context) (*models.MaxSessionResponse, error) {
	maxUser, err := s.currentMaxUser(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.sessionResponse(ctx, maxUser.ID, maxUser.MaxUserID, maxUser.Username, maxUser.AppUsername, maxUser.CustomerID, maxUser.CustomerSalePlaceID)
}

func (s *MaxMiniAppService) FindCustomer(ctx context.Context, input models.MaxCustomerLookupRequest) (*models.MaxCustomerLookupResult, error) {
	if _, err := s.currentMaxUser(ctx, false); err != nil {
		return nil, err
	}
	inn, appUsername, err := validateMaxCustomerLookup(input)
	if err != nil {
		return nil, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)

	result := models.MaxCustomerLookupResult{}
	if err := poolConn.Conn().QueryRow(ctx, `
		SELECT id, name, inn, NULLIF(btrim(kpp), '')
		FROM public.customers
		WHERE is_active AND inn = $1
	`, inn).Scan(&result.ID, &result.Name, &result.INN, &result.KPP); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("customer not found", map[string]any{"inn": inn})
		}
		return nil, fmt.Errorf("find customer by INN: %w", err)
	}
	if err := s.Session.Set(maxRegistrationCustomerKey, result.ID); err != nil {
		return nil, fmt.Errorf("store registration customer: %w", err)
	}
	if err := s.Session.Set(maxRegistrationAppUsernameKey, appUsername); err != nil {
		return nil, fmt.Errorf("store registration app username: %w", err)
	}
	if err := s.Session.Flush(); err != nil {
		return nil, fmt.Errorf("flush registration customer: %w", err)
	}
	return &result, nil
}

func (s *MaxMiniAppService) SalePlaces(ctx context.Context, customerID int) ([]*models.MaxSalePlace, error) {
	if _, err := s.currentMaxUser(ctx, false); err != nil {
		return nil, err
	}
	if customerID <= 0 {
		return nil, webapp.BadRequest("customer_id should be positive", nil)
	}
	var pendingCustomerID int
	if err := s.Session.Get(maxRegistrationCustomerKey, &pendingCustomerID); err != nil || pendingCustomerID != customerID {
		return nil, webapp.Forbidden("customer should be selected by INN first", nil)
	}
	return s.listSalePlaces(ctx, customerID)
}

func (s *MaxMiniAppService) CustomerSalePlaces(ctx context.Context) ([]*models.MaxSalePlace, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	return s.listSalePlaces(ctx, *maxUser.CustomerID)
}

func (s *MaxMiniAppService) OrderDateLimits(ctx context.Context) (*models.MaxOrderDateLimits, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for order date limits: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	policy, err := loadCustomerOrderDatePolicy(ctx, poolConn.Conn(), *maxUser.CustomerID, time.Now())
	if err != nil {
		return nil, err
	}
	return policy.maxOrderDateLimits(), nil
}

func (s *MaxMiniAppService) listSalePlaces(ctx context.Context, customerID int) ([]*models.MaxSalePlace, error) {
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)

	rows, err := poolConn.Conn().Query(ctx, `
		SELECT id, name, address
		FROM public.customer_sale_places
		WHERE customer_id = $1 AND is_active
		ORDER BY lower(name), id
	`, customerID)
	if err != nil {
		return nil, fmt.Errorf("list customer sale places: %w", err)
	}
	defer rows.Close()

	result := make([]*models.MaxSalePlace, 0)
	for rows.Next() {
		item := &models.MaxSalePlace{}
		if err := rows.Scan(&item.ID, &item.Name, &item.Address); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *MaxMiniAppService) Register(ctx context.Context, input models.MaxRegistrationRequest) (*models.MaxSessionResponse, error) {
	maxUser, err := s.currentMaxUser(ctx, false)
	if err != nil {
		return nil, err
	}
	if input.CustomerID <= 0 || input.CustomerSalePlaceID <= 0 {
		return nil, webapp.BadRequest("customer_id and customer_sale_place_id are required", nil)
	}
	var pendingCustomerID int
	if err := s.Session.Get(maxRegistrationCustomerKey, &pendingCustomerID); err != nil || pendingCustomerID != input.CustomerID {
		return nil, webapp.Forbidden("customer should be selected by INN first", nil)
	}
	var pendingAppUsername string
	if err := s.Session.Get(maxRegistrationAppUsernameKey, &pendingAppUsername); err != nil {
		return nil, webapp.Forbidden("app username should be provided during customer selection", nil)
	}
	pendingAppUsername = strings.TrimSpace(pendingAppUsername)
	if pendingAppUsername == "" {
		return nil, webapp.Forbidden("app username should be provided during customer selection", nil)
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)

	var customerName, salePlaceName string
	if err := poolConn.Conn().QueryRow(ctx, `
		SELECT c.name, sp.name
		FROM public.customers AS c
		JOIN public.customer_sale_places AS sp ON sp.customer_id = c.id
		WHERE c.id = $1
			AND sp.id = $2
			AND c.is_active
			AND sp.is_active
	`, input.CustomerID, input.CustomerSalePlaceID).Scan(&customerName, &salePlaceName); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.BadRequest("customer or sale place is invalid", nil)
		}
		return nil, err
	}

	if _, err := poolConn.Conn().Exec(ctx, `
		UPDATE public.max_users
		SET customer_id = $2, customer_sale_place_id = $3, app_username = $4, is_active = true
		WHERE id = $1
	`, maxUser.ID, input.CustomerID, input.CustomerSalePlaceID, pendingAppUsername); err != nil {
		return nil, fmt.Errorf("register MAX user: %w", err)
	}

	return &models.MaxSessionResponse{
		Registered: true,
		User: &models.MaxSessionUser{
			ID:                  maxUser.ID,
			MaxUserID:           maxUser.MaxUserID,
			Username:            maxUser.Username,
			AppUsername:         pendingAppUsername,
			CustomerID:          &input.CustomerID,
			CustomerSalePlaceID: &input.CustomerSalePlaceID,
		},
		Customer:  newRef(input.CustomerID, customerName),
		SalePlace: newRef(input.CustomerSalePlaceID, salePlaceName),
	}, nil
}

func (s *MaxMiniAppService) Catalogue(ctx context.Context, scope models.MaxCatalogueScope) ([]*models.MaxCatalogueItem, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if scope == "" {
		scope = models.MaxCatalogueScopeAll
	}
	if scope != models.MaxCatalogueScopeAll && scope != models.MaxCatalogueScopeHistory {
		return nil, webapp.BadRequest("invalid catalogue scope", map[string]any{"scope": scope})
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)

	query := `
		SELECT p.id, p.parent_id, p.name, p.description, p.measure_unit_id,
			CASE 
				WHEN mu.id IS NULL THEN NULL 
				ELSE 
					jsonb_build_object(
						'keys', jsonb_build_object('id', mu.id), 
						'descr', mu.name
					) 
				END,
			p.is_group
		FROM public.products AS p
		LEFT JOIN public.measure_units AS mu ON mu.id = p.measure_unit_id
		WHERE 
			p.is_active 
			AND (
				p.is_group OR (
					p.ref_1c IS NOT NULL 
					AND p.ref_1c->>'id' IS NOT NULL
					AND btrim(ref_1c->>'id') <> ''
				) 
			)
	`
	args := []any{}
	if scope == models.MaxCatalogueScopeHistory {
		query = `
			WITH RECURSIVE history_products AS (
				SELECT DISTINCT oi.product_id AS id
				FROM public.order_items oi
				JOIN public.orders o ON o.id = oi.order_id
				WHERE o.customer_id = $1
			), included AS (
				SELECT p.id, p.parent_id
				FROM public.products p
				JOIN history_products h ON h.id = p.id
				WHERE p.is_active
				UNION
				SELECT parent.id, parent.parent_id
				FROM public.products parent
				JOIN included child ON child.parent_id = parent.id
				WHERE parent.is_active
			)
			SELECT p.id, p.parent_id, p.name, p.description, p.measure_unit_id,
				CASE WHEN mu.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', mu.id), 'descr', mu.name) END,
				p.is_group
			FROM public.products p
			JOIN included i ON i.id = p.id
			LEFT JOIN public.measure_units mu ON mu.id = p.measure_unit_id
		`
		args = append(args, *maxUser.CustomerID)
	}
	query += ` ORDER BY p.sort_order, lower(p.name), p.id`

	rows, err := poolConn.Conn().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load MAX catalogue: %w", err)
	}
	defer rows.Close()

	all := make([]*models.MaxCatalogueItem, 0)
	byID := make(map[int]*models.MaxCatalogueItem)
	byParent := make(map[int][]*models.MaxCatalogueItem)
	for rows.Next() {
		item := &models.MaxCatalogueItem{}
		if err := rows.Scan(&item.ID, &item.ParentID, &item.Name, &item.Description, &item.MeasureUnitID, &item.MeasureUnit, &item.IsGroup); err != nil {
			return nil, err
		}
		all = append(all, item)
		byID[item.ID] = item
		if item.ParentID != nil {
			byParent[*item.ParentID] = append(byParent[*item.ParentID], item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	roots := make([]*models.MaxCatalogueItem, 0)
	for _, item := range all {
		item.Children = byParent[item.ID]
		if item.ParentID == nil || byID[*item.ParentID] == nil {
			roots = append(roots, item)
		}
	}
	return roots, nil
}

func (s *MaxMiniAppService) Orders(ctx context.Context, lastOrderID *int) ([]*models.MaxOrderListItem, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)

	orderCond := ""
	queryParams := []any{*maxUser.CustomerID}
	if lastOrderID != nil {
		queryParams = append(queryParams, *lastOrderID)
		orderCond = " AND o.id < $2"
	}

	rows, err := poolConn.Conn().Query(ctx, fmt.Sprintf(`
		SELECT 
			o.id, 
			o.version, 
			o.for_date, 
			o.number_1c, 
			st.code, 
			st.name,
			CASE 
				WHEN mu.id IS NULL THEN NULL 
				ELSE 
					jsonb_build_object(
						'keys', jsonb_build_object('id', mu.id), 
						'descr', COALESCE(
							NULLIF(btrim(mu.app_username), ''),
							NULLIF(btrim(mu.username), ''),
							mu.max_user_id::text
						)
					) 
			END,
			jsonb_build_object(
				'keys', jsonb_build_object('id', sp.id), 
				'descr', sp.name
			),
			COUNT(oi.id)::int,
			COALESCE(
				BOOL_OR(oi.quant IS DISTINCT FROM oi.quant_required),
				FALSE
			)

		FROM public.orders o
		JOIN public.order_statuses st ON st.id = o.status_id
		JOIN public.customer_sale_places sp ON sp.id = o.customer_sale_place_id
		LEFT JOIN public.max_users mu ON mu.id = o.customer_user_id
		LEFT JOIN public.order_items oi ON oi.order_id = o.id
		WHERE o.customer_id = $1 %s
		GROUP BY 
			o.id, 
			st.id, 
			mu.id, 
			sp.id
		ORDER BY 
			o.id DESC
		LIMIT %d
	`, orderCond, orderListCount), queryParams...,
	)
	if err != nil {
		return nil, fmt.Errorf("list MAX orders: %w", err)
	}
	defer rows.Close()

	result := make([]*models.MaxOrderListItem, 0)
	for rows.Next() {
		item := &models.MaxOrderListItem{}
		if err := rows.Scan(
			&item.ID,
			&item.Version,
			&item.ForDate,
			&item.Number1C,
			&item.StatusCode,
			&item.StatusName,
			&item.CustomerUser,
			&item.CustomerSalePlace,
			&item.ItemsCount,
			&item.HasQuantDifference,
		); err != nil {
			return nil, err
		}
		item.CanChange = item.StatusCode == "new"
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *MaxMiniAppService) OrderDetail(ctx context.Context, id int) (*models.MaxOrderDetail, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("order id should be positive", nil)
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)
	return fetchMaxOrderDetail(ctx, poolConn.Conn(), id, *maxUser.CustomerID)
}

func (s *MaxMiniAppService) CreateOrder(ctx context.Context, input models.MaxOrderSubmitRequest) (*models.MaxOrderDetail, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := validateMaxOrderSubmit(&input); err != nil {
		return nil, err
	}
	forDate, err := parseMaxDate(input.ForDate)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	var result *models.MaxOrderDetail
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := validateMaxSalePlace(ctx, tx, *maxUser.CustomerID, input.CustomerSalePlaceID); err != nil {
			return err
		}
		datePolicy, err := loadCustomerOrderDatePolicy(ctx, tx, *maxUser.CustomerID, now)
		if err != nil {
			return err
		}
		if err := datePolicy.validate(forDate); err != nil {
			return err
		}
		statusID, err := orderStatusID(ctx, tx, "new")
		if err != nil {
			return err
		}
		var orderID int
		var version int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO public.orders (for_date, customer_id, customer_sale_place_id, customer_user_id, status_id, comment_customer)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, version
		`, forDate, *maxUser.CustomerID, input.CustomerSalePlaceID, maxUser.ID, statusID, trimOptionalString(input.CommentCustomer)).Scan(&orderID, &version); err != nil {
			return err
		}
		if err := replaceMaxOrderItems(ctx, tx, orderID, input.Items); err != nil {
			return err
		}
		if err := rebuildProductRegisterActions(ctx, tx, orderRecorderType, orderID); err != nil {
			return err
		}
		if _, err := enqueueOrder1CSync(ctx, tx, orderID, version); err != nil {
			return err
		}
		result, err = fetchMaxOrderDetail(ctx, tx, orderID, *maxUser.CustomerID)
		if err != nil {
			return err
		}
		return enqueueOrderResponsibleNotifications(
			ctx,
			tx,
			notificationEventOrderSubmitted,
			orderNotificationDataFromMaxDetail(result, false),
		)
	}); err != nil {
		return nil, fmt.Errorf("create MAX order: %w", err)
	}
	return result, nil
}

func (s *MaxMiniAppService) UpdateOrder(ctx context.Context, input models.MaxOrderUpdateRequest) (*models.MaxOrderDetail, error) {
	maxUser, err := s.currentMaxUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.Version <= 0 {
		return nil, webapp.BadRequest("order id and version are required", nil)
	}
	submit := models.MaxOrderSubmitRequest{CustomerSalePlaceID: input.CustomerSalePlaceID, ForDate: input.ForDate, CommentCustomer: input.CommentCustomer, Items: input.Items}
	if err := validateMaxOrderSubmit(&submit); err != nil {
		return nil, err
	}
	forDate, err := parseMaxDate(input.ForDate)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	var result *models.MaxOrderDetail
	if err := withPrimaryTransaction(ctx, s.DB, func(tx ds.Tx) error {
		if err := validateMaxSalePlace(ctx, tx, *maxUser.CustomerID, input.CustomerSalePlaceID); err != nil {
			return err
		}
		var currentVersion int64
		var statusCode string
		var currentForDate time.Time
		if err := tx.QueryRow(ctx, `
			SELECT o.version, st.code, o.for_date
			FROM public.orders o
			JOIN public.order_statuses st ON st.id = o.status_id
			WHERE o.id = $1 AND o.customer_id = $2
			FOR UPDATE OF o
		`, input.ID, *maxUser.CustomerID).Scan(&currentVersion, &statusCode, &currentForDate); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.NotFound("order not found", map[string]any{"id": input.ID})
			}
			return err
		}
		if statusCode != "new" {
			return webapp.Conflict("order can no longer be changed by customer", map[string]any{"status": statusCode})
		}
		if currentVersion != input.Version {
			return webapp.Conflict("order was changed by another request", map[string]any{"expected_version": input.Version, "current_version": currentVersion})
		}
		// Quantity and comment edits keep their previously accepted schedule.
		// Reapply today's policy only when the customer changes the civil date.
		if !sameOrderDate(currentForDate, forDate) {
			datePolicy, err := loadCustomerOrderDatePolicy(ctx, tx, *maxUser.CustomerID, now)
			if err != nil {
				return err
			}
			if err := datePolicy.validate(forDate); err != nil {
				return err
			}
		}
		var newVersion int64
		if err := tx.QueryRow(ctx, `
			UPDATE public.orders
			SET for_date = $2, comment_customer = $3, customer_user_id = $4, customer_sale_place_id = $5, version = version + 1
			WHERE id = $1
			RETURNING version
		`, input.ID, forDate, trimOptionalString(input.CommentCustomer), maxUser.ID, input.CustomerSalePlaceID).Scan(&newVersion); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.order_items WHERE order_id = $1`, input.ID); err != nil {
			return err
		}
		if err := replaceMaxOrderItems(ctx, tx, input.ID, input.Items); err != nil {
			return err
		}
		if err := rebuildProductRegisterActions(ctx, tx, orderRecorderType, input.ID); err != nil {
			return err
		}
		if _, err := enqueueOrder1CSync(ctx, tx, input.ID, newVersion); err != nil {
			return err
		}
		result, err = fetchMaxOrderDetail(ctx, tx, input.ID, *maxUser.CustomerID)
		if err != nil {
			return err
		}
		return enqueueOrderResponsibleNotifications(
			ctx,
			tx,
			notificationEventOrderSubmitted,
			orderNotificationDataFromMaxDetail(result, true),
		)
	}); err != nil {
		return nil, fmt.Errorf("update MAX order: %w", err)
	}
	return result, nil
}

type currentMaxUser struct {
	ID                  int
	MaxUserID           int64
	Username            *string
	AppUsername         string
	CustomerID          *int
	CustomerSalePlaceID *int
}

func (s *MaxMiniAppService) currentMaxUser(ctx context.Context, registered bool) (*currentMaxUser, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	var id int
	if err := s.Session.Get(maxSessionUserKey, &id); err != nil || id <= 0 {
		return nil, apperrors.SessionRequired()
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)
	result := &currentMaxUser{ID: id}
	if err := poolConn.Conn().QueryRow(ctx, `
		SELECT max_user_id, username, app_username, customer_id, customer_sale_place_id
		FROM public.max_users
		WHERE id = $1 AND is_active
	`, id).Scan(&result.MaxUserID, &result.Username, &result.AppUsername, &result.CustomerID, &result.CustomerSalePlaceID); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, apperrors.SessionRequired()
		}
		return nil, err
	}
	if registered && (result.CustomerID == nil || result.CustomerSalePlaceID == nil) {
		return nil, webapp.Forbidden("MAX user is not registered", nil)
	}
	return result, nil
}

func (s *MaxMiniAppService) sessionResponse(ctx context.Context, id int, maxUserID int64, username *string, appUsername string, customerID, salePlaceID *int) (*models.MaxSessionResponse, error) {
	result := &models.MaxSessionResponse{
		Registered: customerID != nil && salePlaceID != nil,
		User:       &models.MaxSessionUser{ID: id, MaxUserID: maxUserID, Username: username, AppUsername: appUsername, CustomerID: customerID, CustomerSalePlaceID: salePlaceID},
	}
	if !result.Registered {
		return result, nil
	}
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, err
	}
	defer s.DB.Release(poolConn, connID)
	var customerName, salePlaceName string
	if err := poolConn.Conn().QueryRow(ctx, `SELECT c.name, sp.name FROM public.customers c JOIN public.customer_sale_places sp ON sp.id = $2 AND sp.customer_id = c.id WHERE c.id = $1`, *customerID, *salePlaceID).Scan(&customerName, &salePlaceName); err != nil {
		return nil, err
	}
	result.Customer = newRef(*customerID, customerName)
	result.SalePlace = newRef(*salePlaceID, salePlaceName)
	return result, nil
}

func validateMaxInitData(initData, botToken string, maxAge time.Duration, now time.Time) (*maxInitUser, []byte, error) {
	if initData == "" {
		return nil, nil, fmt.Errorf("init_data is required")
	}
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, nil, fmt.Errorf("parse init_data: %w", err)
	}
	for key, list := range values {
		if len(list) != 1 {
			return nil, nil, fmt.Errorf("parameter %q should occur exactly once", key)
		}
	}
	hashValue := values.Get("hash")
	if hashValue == "" {
		return nil, nil, fmt.Errorf("hash is required")
	}
	keys := make([]string, 0, len(values)-1)
	for key := range values {
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	launchParams := strings.Join(parts, "\n")
	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(botToken))
	secretKey := secretMAC.Sum(nil)
	signatureMAC := hmac.New(sha256.New, secretKey)
	_, _ = signatureMAC.Write([]byte(launchParams))
	expectedHash := signatureMAC.Sum(nil)
	actualHash, err := hex.DecodeString(hashValue)
	if err != nil || !hmac.Equal(expectedHash, actualHash) {
		return nil, nil, fmt.Errorf("signature mismatch")
	}
	if maxAge > 0 {
		unix, parseErr := parseUnix(values.Get("auth_date"))
		if parseErr != nil {
			return nil, nil, parseErr
		}
		issued := time.Unix(unix, 0)
		if issued.After(now.Add(5*time.Minute)) || now.Sub(issued) > maxAge {
			return nil, nil, fmt.Errorf("init_data has expired")
		}
	}
	rawUser := []byte(values.Get("user"))
	var user maxInitUser
	if err := json.Unmarshal(rawUser, &user); err != nil {
		return nil, nil, fmt.Errorf("decode user: %w", err)
	}
	if user.ID <= 0 {
		return nil, nil, fmt.Errorf("user id is required")
	}
	return &user, rawUser, nil
}

func parseUnix(value string) (int64, error) {
	unix, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || unix <= 0 {
		return 0, fmt.Errorf("invalid auth_date")
	}
	return unix, nil
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func initialMaxUsernames(value *string) (*string, string) {
	username := normalizedOptionalString(value)
	if username == nil {
		return nil, defaultMaxAppUsername
	}
	return username, *username
}

func validateMaxCustomerLookup(input models.MaxCustomerLookupRequest) (string, string, error) {
	inn := strings.TrimSpace(input.INN)
	if inn == "" {
		return "", "", webapp.BadRequest("inn is required", nil)
	}
	appUsername := strings.TrimSpace(input.AppUsername)
	if appUsername == "" {
		return "", "", webapp.BadRequest("app_username is required", nil)
	}
	return inn, appUsername, nil
}

func newRef(id int, descr string) *models.Ref {
	ref := &models.Ref{Descr: descr}
	ref.Keys.ID = id
	return ref
}

func validateMaxOrderSubmit(input *models.MaxOrderSubmitRequest) error {
	if input == nil || strings.TrimSpace(input.ForDate) == "" {
		return webapp.BadRequest("for_date is required", nil)
	}
	if input.CustomerSalePlaceID <= 0 {
		return webapp.BadRequest("customer_sale_place_id should be positive", nil)
	}
	if len(input.Items) == 0 || len(input.Items) > orderDocumentMaxItems {
		return webapp.BadRequest("order should contain between 1 and 1000 items", nil)
	}
	seen := make(map[int]struct{}, len(input.Items))
	for i, item := range input.Items {
		if item.ProductID <= 0 || item.QuantRequired <= 0 {
			return webapp.BadRequest(fmt.Sprintf("items[%d] is invalid", i), nil)
		}
		if _, ok := seen[item.ProductID]; ok {
			return webapp.BadRequest(fmt.Sprintf("items[%d] product is duplicated", i), nil)
		}
		seen[item.ProductID] = struct{}{}
	}
	input.CommentCustomer = trimOptionalString(input.CommentCustomer)
	return nil
}

func validateMaxSalePlace(ctx context.Context, db ds.Querier, customerID, salePlaceID int) error {
	var exists int
	if err := db.QueryRow(ctx, `
		SELECT 1
		FROM public.customer_sale_places AS sp
		JOIN public.customers AS c ON c.id = sp.customer_id
		WHERE sp.id = $1
			AND sp.customer_id = $2
			AND sp.is_active
			AND c.is_active
	`, salePlaceID, customerID).Scan(&exists); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return webapp.BadRequest("customer sale place is invalid", map[string]any{"customer_sale_place_id": salePlaceID})
		}
		return err
	}
	return nil
}

func parseMaxDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, webapp.BadRequest("for_date should use YYYY-MM-DD format", nil)
	}
	return parsed, nil
}

func replaceMaxOrderItems(ctx context.Context, tx ds.Querier, orderID int, items []models.MaxOrderItemInput) error {
	for index, item := range items {
		var measureUnitID int
		if err := tx.QueryRow(ctx, `SELECT measure_unit_id FROM public.products WHERE id = $1 AND is_active AND NOT is_group AND measure_unit_id IS NOT NULL`, item.ProductID).Scan(&measureUnitID); err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				return webapp.BadRequest("product is not available for ordering", map[string]any{"product_id": item.ProductID})
			}
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO public.order_items (line_num, order_id, product_id, measure_unit_id, quant_required, quant)
			VALUES ($1, $2, $3, $4, $5, $5)
		`, index+1, orderID, item.ProductID, measureUnitID, item.QuantRequired); err != nil {
			return err
		}
	}
	return nil
}

func orderStatusID(ctx context.Context, tx ds.Querier, code string) (int, error) {
	var id int
	if err := tx.QueryRow(ctx, `SELECT id FROM public.order_statuses WHERE code = $1`, code).Scan(&id); err != nil {
		return 0, fmt.Errorf("load order status %q: %w", code, err)
	}
	return id, nil
}

func fetchMaxOrderDetail(ctx context.Context, db ds.Querier, id, customerID int) (*models.MaxOrderDetail, error) {
	rows, err := db.Query(ctx, `
		SELECT o.id, o.version, o.for_date, o.number_1c, o.ref_1c, st.code, st.name,
			c.id, c.name, sp.id, sp.name,
			CASE WHEN mu.id IS NULL THEN NULL ELSE jsonb_build_object(
				'keys', jsonb_build_object('id', mu.id),
				'descr', COALESCE(
					NULLIF(btrim(mu.app_username), ''),
					NULLIF(btrim(mu.username), ''),
					mu.max_user_id::text
				)
			) END,
			o.comment_customer,
			oi.id, oi.product_id, p.name, oi.measure_unit_id,
			CASE WHEN unit.id IS NULL THEN NULL ELSE jsonb_build_object('keys', jsonb_build_object('id', unit.id), 'descr', unit.name) END,
			oi.quant_required::double precision, COALESCE(oi.quant, 0)::double precision
		FROM public.orders o
		JOIN public.order_statuses st ON st.id = o.status_id
		JOIN public.customers c ON c.id = o.customer_id
		JOIN public.customer_sale_places sp ON sp.id = o.customer_sale_place_id
		LEFT JOIN public.max_users mu ON mu.id = o.customer_user_id
		LEFT JOIN public.order_items oi ON oi.order_id = o.id
		LEFT JOIN public.products p ON p.id = oi.product_id
		LEFT JOIN public.measure_units unit ON unit.id = oi.measure_unit_id
		WHERE o.id = $1 AND o.customer_id = $2
		ORDER BY oi.line_num, oi.id
	`, id, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result *models.MaxOrderDetail
	for rows.Next() {
		var orderID, productID, itemID, measureUnitID *int
		var customerRef, salePlaceRef models.Ref
		var customerIDVal, salePlaceID int
		var itemProductName *string
		var version int64
		var forDate time.Time
		var number1C *string
		var ref1C *models.Ref1c
		var statusCode, statusName string
		var customerUser, measureUnit *models.Ref
		var comment *string
		var quantReq, quant *float64
		var scannedOrderID int
		if err := rows.Scan(&scannedOrderID, &version, &forDate, &number1C, &ref1C, &statusCode, &statusName,
			&customerIDVal, &customerRef.Descr, &salePlaceID, &salePlaceRef.Descr, &customerUser, &comment,
			&itemID, &productID, &itemProductName, &measureUnitID, &measureUnit, &quantReq, &quant); err != nil {
			return nil, err
		}
		orderID = &scannedOrderID
		if result == nil {
			customerRef.Keys.ID = customerIDVal
			salePlaceRef.Keys.ID = salePlaceID
			result = &models.MaxOrderDetail{ID: *orderID, Version: version, ForDate: forDate, Number1C: number1C, Ref1C: ref1C, StatusCode: statusCode, StatusName: statusName, Customer: customerRef, CustomerSalePlace: salePlaceRef, CustomerUser: customerUser, CommentCustomer: comment, CanChange: statusCode == "new", Items: make([]*models.MaxOrderItem, 0)}
		}
		if itemID != nil && productID != nil && measureUnitID != nil && quantReq != nil && quant != nil {
			product := models.Ref{Descr: ""}
			product.Keys.ID = *productID
			if itemProductName != nil {
				product.Descr = *itemProductName
			}
			result.Items = append(result.Items, &models.MaxOrderItem{ID: *itemID, ProductID: *productID, Product: product, MeasureUnitID: *measureUnitID, MeasureUnit: measureUnit, QuantRequired: *quantReq, Quant: *quant})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, webapp.NotFound("order not found", map[string]any{"id": id})
	}
	return result, nil
}
