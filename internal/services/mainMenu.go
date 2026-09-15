package services

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/types"
	"github.com/dronm/session"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

type MainMenuService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
	Cache   MainMenuCache
}

func NewMainMenuService(ctx webapp.ServiceContext, cache MainMenuCache) any {
	return &MainMenuService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
		Cache:   cache,
	}
}

func RegisterMainMenuService(cache MainMenuCache) {
	webapp.MustRegisterService(
		"MainMenu",
		&MainMenuService{},
		func(ctx webapp.ServiceContext) any {
			return NewMainMenuService(ctx, cache)
		},
		webapp.WithCRUDNotifications(),
	)
}

func (s *MainMenuService) Create(
	ctx context.Context,
	input modelbind.ModelInput[*models.MainMenu],
) (map[string]any, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input.Model == nil {
		return nil, webapp.BadRequest("main menu input is required", nil)
	}
	if err := validateMainMenuOwner(input.Model.RoleID, input.Model.UserID); err != nil {
		return nil, err
	}

	result, err := webapp.InsertModelInput(ctx, s.DB, input, nil)
	if err != nil {
		return nil, fmt.Errorf("insert main menu item: %w", err)
	}

	s.invalidateCache(ctx)
	return result, nil
}

func (s *MainMenuService) Update(
	ctx context.Context,
	input webapp.UpdateByKeysInput[*models.MainMenuKey, *models.MainMenu],
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if input.Keys == nil || input.Keys.ID <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("main menu id is required", nil)
	}

	rowsAffected, err := webapp.UpdateModelInput(ctx, s.DB, input.Keys, input.Input, nil)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("update main menu item %d: %w", input.Keys.ID, err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"main menu item not found",
			map[string]any{"id": input.Keys.ID},
		)
	}

	s.invalidateCache(ctx)
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *MainMenuService) Delete(
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
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("main menu id is required", nil)
	}

	rowsAffected, err := webapp.DeleteModel(
		ctx,
		s.DB,
		[]types.DBModel{models.MainMenuKey{ID: id}},
		nil,
	)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete main menu item %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"main menu item not found",
			map[string]any{"id": id},
		)
	}

	s.invalidateCache(ctx)
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *MainMenuService) Detail(ctx context.Context, id int) (*models.MainMenu, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("main menu id is required", nil)
	}

	result, err := webapp.FetchModel(ctx, s.DB, models.MainMenuKey{ID: id}, &models.MainMenu{})
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("main menu item not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch main menu item %d: %w", id, err)
	}

	return result, nil
}

func (s *MainMenuService) List(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.MainMenuList], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.MainMenuList]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.MainMenuList]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.MainMenuList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.MainMenuList]{}, fmt.Errorf("fetch main menu collection: %w", err)
	}

	return wmodels.CollectionResponse[*models.MainMenuList]{Rows: rows, Agg: total}, nil
}

func (s *MainMenuService) ForUser(ctx context.Context) ([]*models.MainMenuForUser, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	gob.Register(models.UserLogin{})
	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil {
		return nil, fmt.Errorf("read logged user from session: %w", err)
	}
	if user.ID <= 0 {
		return nil, webapp.Unauthorized("logged user is required", nil)
	}

	cacheVersion := int64(0)
	if s.Cache != nil {
		menu, version, err := s.Cache.Get(ctx, user.ID, user.RoleID)
		if err == nil {
			return menu, nil
		}
		if errors.Is(err, ErrMainMenuCacheMiss) {
			cacheVersion = version
		} else {
			slog.Warn("main menu cache read failed", "user_id", user.ID, "error", err)
		}
	}

	menu, err := s.fetchForUser(ctx, user.ID, user.RoleID)
	if err != nil {
		return nil, err
	}

	if s.Cache != nil && cacheVersion > 0 {
		if err := s.Cache.Set(ctx, user.ID, user.RoleID, cacheVersion, menu); err != nil {
			slog.Warn("main menu cache write failed", "user_id", user.ID, "error", err)
		}
	}

	return menu, nil
}

func (s *MainMenuService) RouteAutocomplete(
	ctx context.Context,
	input models.MainMenuRouteAutocompleteInput,
) ([]models.MainMenuRouteOption, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	input.Query = strings.TrimSpace(input.Query)
	if input.Limit <= 0 {
		input.Limit = 20
	}
	if input.Limit > 50 {
		input.Limit = 50
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for route autocomplete: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	query := `
		SELECT
			id,
			name,
			path,
			descr,
			section,
			icon
		FROM public.application_routes
		WHERE is_active
			AND menu_available
	`
	args := make([]any, 0, 2)
	if input.Query != "" {
		query += `
			AND (
				lower(descr) LIKE '%' || lower($1) || '%'
				OR lower(name) LIKE '%' || lower($1) || '%'
			)
		`
		args = append(args, input.Query)
	}
	query += ` ORDER BY section, descr, name LIMIT $` + fmt.Sprintf("%d", len(args)+1)
	args = append(args, input.Limit)

	rows, err := poolConn.Conn().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select application routes for menu autocomplete: %w", err)
	}
	defer rows.Close()

	result := make([]models.MainMenuRouteOption, 0)
	for rows.Next() {
		var item models.MainMenuRouteOption
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Path,
			&item.Descr,
			&item.Section,
			&item.Icon,
		); err != nil {
			return nil, fmt.Errorf("scan application route option: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application route options: %w", err)
	}

	return result, nil
}

func (s *MainMenuService) fetchForUser(
	ctx context.Context,
	userID int,
	roleID models.RoleID,
) ([]*models.MainMenuForUser, error) {
	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get primary connection for user menu: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	rows, err := poolConn.Conn().Query(ctx, `
		WITH menu_scope AS (
			SELECT EXISTS (
				SELECT 1
				FROM public.main_menus AS user_menu
				LEFT JOIN public.application_routes AS user_route ON user_route.id = user_menu.route_id
				WHERE user_menu.user_id = $1
					AND user_menu.parent_id IS NULL
					AND user_menu.is_active
					AND (user_menu.route_id IS NULL OR user_route.is_active)
			) AS use_user_menu
		)
		SELECT
			m.id,
			m.parent_id,
			m.caption,
			COALESCE(route.name, ''),
			COALESCE(m.icon, route.icon)
		FROM public.main_menus AS m
		CROSS JOIN menu_scope AS scope
		LEFT JOIN public.application_routes AS route ON route.id = m.route_id
		WHERE m.is_active
			AND (m.route_id IS NULL OR route.is_active)
			AND (
				(scope.use_user_menu AND m.user_id = $1)
				OR
				(NOT scope.use_user_menu AND m.role_id = $2)
			)
		ORDER BY m.parent_id NULLS FIRST, m.sort_order, m.id
	`, userID, roleID)
	if err != nil {
		return nil, fmt.Errorf("select user main menu: %w", err)
	}
	defer rows.Close()

	type menuRow struct {
		ID       int
		ParentID *int
		Caption  string
		Route    string
		Icon     *string
	}

	flat := make([]menuRow, 0)
	for rows.Next() {
		var row menuRow
		if err := rows.Scan(&row.ID, &row.ParentID, &row.Caption, &row.Route, &row.Icon); err != nil {
			return nil, fmt.Errorf("scan user main menu: %w", err)
		}
		flat = append(flat, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user main menu: %w", err)
	}

	items := make(map[int]*models.MainMenuForUser, len(flat))
	for _, row := range flat {
		items[row.ID] = &models.MainMenuForUser{
			ID:      row.ID,
			Caption: row.Caption,
			Route:   row.Route,
			Icon:    row.Icon,
		}
	}

	menu := make([]*models.MainMenuForUser, 0)
	for _, row := range flat {
		item := items[row.ID]
		if row.ParentID == nil {
			menu = append(menu, item)
			continue
		}

		parent, ok := items[*row.ParentID]
		if !ok {
			continue
		}
		parent.Children = append(parent.Children, item)
	}

	return menu, nil
}

func validateMainMenuOwner(roleID *models.RoleID, userID *int) error {
	if (roleID == nil) == (userID == nil) {
		return webapp.BadRequest("exactly one of role_id or user_id is required", nil)
	}
	if userID != nil && *userID <= 0 {
		return webapp.BadRequest("user_id should be positive", nil)
	}
	return nil
}

func (s *MainMenuService) invalidateCache(ctx context.Context) {
	if s.Cache == nil {
		return
	}
	if err := s.Cache.Invalidate(ctx); err != nil {
		slog.Warn("main menu cache invalidation failed", "error", err)
	}
}

func (s *MainMenuService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}
	return nil
}

func (s *MainMenuService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}
	return nil
}
