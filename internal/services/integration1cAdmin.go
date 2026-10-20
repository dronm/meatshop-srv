package services

import (
	"context"

	"github.com/dronm/meatshop/internal/apperrors"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func (s *Integration1CService) Health(ctx context.Context) (integration.AdminResponse, error) {
	return s.adminCommand(ctx, (*integration.Client).Health)
}

func (s *Integration1CService) Status(ctx context.Context) (integration.AdminResponse, error) {
	return s.adminCommand(ctx, (*integration.Client).Status)
}

func (s *Integration1CService) Stop(ctx context.Context) (integration.AdminResponse, error) {
	return s.adminCommand(ctx, (*integration.Client).Stop)
}

func (s *Integration1CService) Start(ctx context.Context) (integration.AdminResponse, error) {
	return s.adminCommand(ctx, (*integration.Client).Start)
}

func (s *Integration1CService) adminCommand(
	ctx context.Context,
	command func(*integration.Client, context.Context) (integration.AdminResponse, error),
) (integration.AdminResponse, error) {
	if err := s.requireSession(); err != nil {
		return integration.AdminResponse{}, err
	}
	var user models.UserLogin
	if err := s.Session.Get("user", &user); err != nil || user.ID <= 0 {
		return integration.AdminResponse{}, apperrors.SessionRequired()
	}
	if user.RoleID != models.RoleIDAdmin {
		return integration.AdminResponse{}, webapp.Forbidden("Administrator access required", nil)
	}
	if err := s.requireClient(); err != nil {
		return integration.AdminResponse{}, err
	}
	response, err := command(s.Client, ctx)
	if err != nil {
		// Do not expose transport errors containing the configured URL or credentials.
		return integration.AdminResponse{}, webapp.Internal("1c gateway request failed or timed out; check its availability and refresh status before repeating start/stop", nil)
	}
	return response, nil
}
