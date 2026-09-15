package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/types"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/config"
	"github.com/dronm/meatshop/internal/models"

	"github.com/dronm/ds/v4"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
	wmodels "github.com/dronm/webapp/models"
)

const userLoginQuery = "USER_LOGIN_QUERY"

type UserService struct {
	DB      ds.Provider
	Session session.Session
	QueryID string
	SessCfg config.SessionConfig
}

func NewUserService(ctx webapp.ServiceContext, cfg config.SessionConfig) any {
	return &UserService{
		DB:      ctx.DB,
		Session: ctx.Session,
		QueryID: ctx.QueryID,
		SessCfg: cfg,
	}
}

func RegisterUserService(cfg config.SessionConfig) {
	webapp.MustRegisterService(
		"User",
		&UserService{},
		func(ctx webapp.ServiceContext) any {
			return NewUserService(ctx, cfg)
		},
		webapp.WithCRUDNotifications(),
	)
}

func (s *UserService) Create(ctx context.Context, input modelbind.ModelInput[*models.User]) (map[string]any, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if input.Model == nil {
		return nil, webapp.BadRequest("user input is required", nil)
	}

	input.Model.Pwd = hashUserPassword(input.Model.Pwd)

	result, err := webapp.InsertModelInput(ctx, s.DB, input, nil)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return result, nil
}

func (s *UserService) Update(
	ctx context.Context,
	input webapp.UpdateByKeysInput[*models.UserKey, *models.UserUpdate],
) (wmodels.RowsAffectedResponse, error) {
	if s.Session == nil {
		return wmodels.RowsAffectedResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	if input.Keys == nil {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user key is required", nil)
	}

	if input.Keys.ID <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user id is required", nil)
	}

	rowsAffected, err := webapp.UpdateModelInput(
		ctx,
		s.DB,
		input.Keys,
		input.Input,
		nil,
	)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("update user %d: %w", input.Keys.ID, err)
	}

	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("user not found", map[string]any{
			"id": input.Keys.ID,
		})
	}

	return wmodels.RowsAffectedResponse{
		RowsAffected: rowsAffected,
	}, nil
}

func (s *UserService) Delete(ctx context.Context, id int) (wmodels.RowsAffectedResponse, error) {
	if s.Session == nil {
		return wmodels.RowsAffectedResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if id <= 0 {
		return wmodels.RowsAffectedResponse{}, webapp.BadRequest("user id is required", nil)
	}

	rowsAffected, err := webapp.DeleteModel(
		ctx,
		s.DB,
		[]types.DBModel{models.UserKey{ID: id}},
		nil,
	)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("delete user %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("user not found", map[string]any{"id": id})
	}

	return wmodels.RowsAffectedResponse{RowsAffected: rowsAffected}, nil
}

func (s *UserService) Detail(ctx context.Context, id int) (*models.UserDetail, error) {
	if s.Session == nil {
		return nil, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, webapp.BadRequest("user id is required", nil)
	}

	result, err := webapp.FetchModel(ctx, s.DB, models.UserKey{ID: id}, &models.UserDetail{})
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("user not found", map[string]any{"id": id})
		}
		return nil, fmt.Errorf("fetch user %d: %w", id, err)
	}

	return result, nil
}

func (s *UserService) List(ctx context.Context, params modelbind.CollectionParams) (wmodels.CollectionResponse[*models.UserList], error) {
	if s.Session == nil {
		return wmodels.CollectionResponse[*models.UserList]{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return wmodels.CollectionResponse[*models.UserList]{}, err
	}

	rows, total, err := webapp.FetchCollectionModel(ctx, s.DB, &models.UserList{}, params)
	if err != nil {
		return wmodels.CollectionResponse[*models.UserList]{}, fmt.Errorf("fetch user collection: %w", err)
	}

	return wmodels.CollectionResponse[*models.UserList]{
		Rows: rows,
		Agg:  total,
	}, nil
}

func (s *UserService) CurrentProfile(ctx context.Context) (*models.UserProfile, error) {
	currentUser, err := s.currentSessionUser()
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	result, err := webapp.FetchModel(
		ctx,
		s.DB,
		models.UserKey{ID: currentUser.ID},
		&models.UserProfile{},
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("user not found", map[string]any{
				"id": currentUser.ID,
			})
		}
		return nil, fmt.Errorf("fetch current user profile: %w", err)
	}

	return result, nil
}

func (s *UserService) UpdateCurrentProfile(
	ctx context.Context,
	input models.UserProfileUpdate,
) (*models.UserProfile, error) {
	currentUser, err := s.currentSessionUser()
	if err != nil {
		return nil, err
	}
	if err := s.requireDB(); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, webapp.BadRequest("user name is required", nil)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return nil, fmt.Errorf("get database connection: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	result := models.UserProfile{}
	err = poolConn.Conn().QueryRow(ctx, `
		UPDATE public.users
		SET name = $1
		WHERE id = $2
		RETURNING id, name
	`, name, currentUser.ID).Scan(
		&result.ID,
		&result.Name,
	)
	if err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.NotFound("user not found", map[string]any{
				"id": currentUser.ID,
			})
		}
		return nil, fmt.Errorf("update current user profile: %w", err)
	}

	currentUser.Name = result.Name
	if err := s.Session.Set("user", currentUser); err != nil {
		return nil, fmt.Errorf("update session user: %w", err)
	}
	if err := s.Session.Flush(); err != nil {
		return nil, fmt.Errorf("flush session user: %w", err)
	}

	return &result, nil
}

func (s *UserService) ChangeCurrentPassword(
	ctx context.Context,
	input models.UserPasswordChange,
) (wmodels.RowsAffectedResponse, error) {
	currentUser, err := s.currentSessionUser()
	if err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}
	if err := s.requireDB(); err != nil {
		return wmodels.RowsAffectedResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("get database connection: %w", err)
	}
	defer s.DB.Release(poolConn, connID)

	tag, err := poolConn.Conn().Exec(ctx, `
		UPDATE public.users
		SET pwd = $1
		WHERE id = $2
	`, hashUserPassword(input.NewPassword), currentUser.ID)
	if err != nil {
		return wmodels.RowsAffectedResponse{}, fmt.Errorf("change current user password: %w", err)
	}

	rowsAffected := tag.RowsAffected()
	if rowsAffected == 0 {
		return wmodels.RowsAffectedResponse{}, webapp.NotFound("user not found", map[string]any{
			"id": currentUser.ID,
		})
	}

	return wmodels.RowsAffectedResponse{
		RowsAffected: rowsAffected,
	}, nil
}

func (s *UserService) Login(
	ctx context.Context,
	input models.UserLoginInput,
) (models.UserLoginResponse, error) {
	if s.Session == nil {
		return models.UserLoginResponse{}, apperrors.SessionRequired()
	}
	if err := s.requireDB(); err != nil {
		return models.UserLoginResponse{}, err
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return models.UserLoginResponse{}, err
	}
	defer s.DB.Release(poolConn, connID)
	conn := poolConn.Conn()

	userRow := models.UserLogin{}
	if _, err := conn.Prepare(ctx,
		userLoginQuery,
		`SELECT
			u.id,
			u.name,
			u.role_id,
			u.create_dt,
			u.pwd,
			COALESCE(u.banned, false),
			(
				SELECT string_agg(b.hash, ',')
				FROM public.login_device_bans AS b
				WHERE b.user_id = u.id
			) AS ban_hash
		FROM public.users AS u
		WHERE u.name = $1 AND u.pwd = md5($2)`,
	); err != nil {
		return models.UserLoginResponse{}, err
	}

	err = conn.QueryRow(ctx, userLoginQuery, input.Model.Name, input.Model.Pwd).Scan(
		&userRow.ID,
		&userRow.Name,
		&userRow.RoleID,
		&userRow.CreateDt,
		&userRow.Pwd,
		&userRow.Banned,
		&userRow.BanHash,
	)
	if err == ds.ErrNoRows {
		// no user with this name &&  pwd
		return models.UserLoginResponse{}, apperrors.InvalidCredentials()
	} else if err != nil {
		return models.UserLoginResponse{}, err
	}

	slog.Debug("UserService.Login, calling loginUser()")
	if err := loginUser(ctx, s.Session, input.UserInf, conn, &userRow); err != nil {
		return models.UserLoginResponse{}, err
	}

	tokenRefresh := ""
	tokenExpires := time.Time{}
	if s.SessCfg.MaxLifeTime > 0 {
		tokenExpires = time.Now().Add(time.Duration(s.SessCfg.MaxLifeTime) * time.Second)
	}

	return models.UserLoginResponse{
		User: &userRow,
		Auth: &wmodels.Auth{
			Token:        s.Session.SessionID(),
			TokenRefresh: tokenRefresh,
			Expires:      tokenExpires,
		},
	}, nil
}

func (s *UserService) Logout(ctx context.Context) error {
	userLogin := models.UserLogin{}
	if err := s.Session.Get("user", &userLogin); err != nil {
		return fmt.Errorf("Session.Get() failed: %v", err)
	}

	poolConn, connID, err := s.DB.GetPrimary(ctx)
	if err != nil {
		return fmt.Errorf("GetPrimary() failed: %v", err)
	}
	defer s.DB.Release(poolConn, connID)
	conn := poolConn.Conn()

	if _, err := conn.Prepare(ctx,
		"user_logout",
		`UPDATE logins 
		SET date_time_out = now() 
		WHERE id = $1`,
	); err != nil {
		return fmt.Errorf("conn.Prepare() failed: %v", err)
	}

	if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
		return fmt.Errorf("conn.Exec() BEGIN failed: %v", err)
	}
	if _, err := conn.Exec(ctx, "user_logout", userLogin.LoginID); err != nil {
		_, _ = conn.Exec(ctx, "ROLLBACK")
		return fmt.Errorf("conn.Exec() failed: %v", err)
	}
	if err := s.Session.Delete(s.Session.SessionID()); err != nil {
		_, _ = conn.Exec(ctx, "ROLLBACK")
		return fmt.Errorf("Session.Delete() failed: %v", err)
	}

	if _, err := conn.Exec(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("conn.Exec() COMMIT failed: %v", err)
	}

	return nil
}

func (s *UserService) currentSessionUser() (models.UserLogin, error) {
	if s.Session == nil {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	user := models.UserLogin{}
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return models.UserLogin{}, apperrors.SessionRequired()
	}

	return user, nil
}

func (s *UserService) requireDB() error {
	if s.DB == nil {
		return webapp.Internal("database is not initialized", nil)
	}

	return nil
}

func hashUserPassword(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}
