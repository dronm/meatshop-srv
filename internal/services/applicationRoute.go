package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/modelbind"
	"github.com/dronm/session"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

type ApplicationRouteService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
	Cache   MainMenuCache
}

func NewApplicationRouteService(ctx webapp.ServiceContext, cache MainMenuCache) any {
	return &ApplicationRouteService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
		Cache:   cache,
	}
}

func RegisterApplicationRouteService(cache MainMenuCache) {
	webapp.MustRegisterService(
		"ApplicationRoute",
		&ApplicationRouteService{},
		func(ctx webapp.ServiceContext) any {
			return NewApplicationRouteService(ctx, cache)
		},
	)
}

func (s *ApplicationRouteService) List(
	ctx context.Context,
	params modelbind.CollectionParams,
) (wmodels.CollectionResponse[*models.ApplicationRoute], error) {
	if err := s.requireSession(); err != nil {
		return wmodels.CollectionResponse[*models.ApplicationRoute]{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.ApplicationRoute]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.ApplicationRoute{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.ApplicationRoute]{}, fmt.Errorf("fetch application route collection: %w", err)
	}

	return wmodels.CollectionResponse[*models.ApplicationRoute]{
		Rows: rows,
		Agg:  total,
	}, nil
}

func (s *ApplicationRouteService) Detail(
	ctx context.Context,
	id int,
) (*models.ApplicationRoute, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("application route id is required", nil)
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		models.ApplicationRouteKey{ID: id},
		&models.ApplicationRoute{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("application route not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch application route %d: %w", id, err)
	}

	return result, nil
}

func (s *ApplicationRouteService) Update(
	ctx context.Context,
	input webapp.UpdateByKeysInput[*models.ApplicationRouteKey, *models.ApplicationRoute],
) (wmodels.RowsAffectedResponse, error) {
	if err := s.requireSession(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if input.Keys == nil || input.Keys.ID <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("application route id is required", nil)
	}

	rowsAffected, err := webapp.UpdateModelInput(ctx, s.DB, input.Keys, input.Input, nil)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("update application route %d: %w", input.Keys.ID, err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound(
			"application route not found",
			map[string]any{"id": input.Keys.ID},
		)
	}

	s.invalidateMenuCache(ctx)
	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *ApplicationRouteService) Sync(
	ctx context.Context,
	input models.ApplicationRouteSyncInput,
) (models.ApplicationRouteSyncResult, error) {
	if err := s.requireSession(); err != nil {
		return models.ApplicationRouteSyncResult{}, err
	}
	if err := s.requireDB(); err != nil {
		return models.ApplicationRouteSyncResult{}, err
	}
	if len(input.Items) == 0 {
		return models.ApplicationRouteSyncResult{}, webapp.BadRequest("application route manifest is empty", nil)
	}
	if len(input.Items) > 2000 {
		return models.ApplicationRouteSyncResult{}, webapp.BadRequest("application route manifest is too large", nil)
	}

	names := make([]string, 0, len(input.Items))
	seen := make(map[string]struct{}, len(input.Items))
	for index := range input.Items {
		item := &input.Items[index]
		item.Name = strings.TrimSpace(item.Name)
		item.Path = strings.TrimSpace(item.Path)
		item.Descr = strings.TrimSpace(item.Descr)
		item.Section = strings.TrimSpace(item.Section)
		if item.Icon != nil {
			value := strings.TrimSpace(*item.Icon)
			if value == "" {
				item.Icon = nil
			} else {
				item.Icon = &value
			}
		}

		if item.Name == "" || item.Path == "" || item.Descr == "" || item.Section == "" {
			return models.ApplicationRouteSyncResult{}, webapp.BadRequest(
				"application route manifest item is incomplete",
				map[string]any{"index": index},
			)
		}
		if _, exists := seen[item.Name]; exists {
			return models.ApplicationRouteSyncResult{}, webapp.BadRequest(
				"application route manifest contains duplicate names",
				map[string]any{"name": item.Name},
			)
		}
		seen[item.Name] = struct{}{}
		names = append(names, item.Name)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.ApplicationRouteSyncResult{}, fmt.Errorf("get primary connection for application route sync: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	tx, err := poolConn.Conn().Begin(ctx)
	if err != nil {
		return models.ApplicationRouteSyncResult{}, fmt.Errorf("begin application route sync: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result := models.ApplicationRouteSyncResult{}
	for _, item := range input.Items {
		var id int
		err := tx.QueryRow(ctx, `
			INSERT INTO public.application_routes (
				name,
				path,
				descr,
				section,
				icon,
				menu_available,
				is_active
			)
			VALUES ($1, $2, $3, $4, $5, $6, true)
			ON CONFLICT (name) DO UPDATE
			SET
				path = EXCLUDED.path,
				descr = EXCLUDED.descr,
				section = EXCLUDED.section,
				icon = EXCLUDED.icon,
				menu_available = EXCLUDED.menu_available,
				is_active = true
			RETURNING id
		`,
			item.Name,
			item.Path,
			item.Descr,
			item.Section,
			item.Icon,
			item.MenuAvailable,
		).Scan(&id)
		if err != nil {
			if errors.Is(err, ds.ErrNoRows) {
				continue
			}
			return models.ApplicationRouteSyncResult{}, fmt.Errorf("synchronize application route %q: %w", item.Name, err)
		}
		result.Registered++
	}

	tag, err := tx.Exec(ctx, `
		UPDATE public.application_routes
		SET is_active = false
		WHERE is_active
			AND NOT (name = ANY($1::text[]))
	`, names)
	if err != nil {
		return models.ApplicationRouteSyncResult{}, fmt.Errorf("deactivate missing application routes: %w", err)
	}
	result.Deactivated = int(tag.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		return models.ApplicationRouteSyncResult{}, fmt.Errorf("commit application route sync: %w", err)
	}

	if result.Registered > 0 || result.Deactivated > 0 {
		s.invalidateMenuCache(ctx)
	}

	return result, nil
}

func (s *ApplicationRouteService) invalidateMenuCache(ctx context.Context) {
	if s.Cache == nil {
		return
	}
	if err := s.Cache.Invalidate(ctx); err != nil {
		slog.Warn("main menu cache invalidation after route sync failed", "error", err)
	}
}

func (s *ApplicationRouteService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}
	return nil
}

func (s *ApplicationRouteService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}
	return nil
}
