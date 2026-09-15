package services

import (
	"context"
	"strings"

	"github.com/dronm/meatshop/internal/apperrors"
	integration "github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/session"
	"github.com/dronm/webapp"
)

type Integration1CService struct {
	Session session.Session
	QueryID string
	Client  *integration.Client
}

func NewIntegration1CService(ctx webapp.ServiceContext, client *integration.Client) any {
	return &Integration1CService{
		Session: ctx.Session,
		QueryID: ctx.QueryID,
		Client:  client,
	}
}

func RegisterIntegration1CService(client *integration.Client) {
	webapp.MustRegisterService(
		"Integration1C",
		&Integration1CService{},
		func(ctx webapp.ServiceContext) any {
			return NewIntegration1CService(ctx, client)
		},
	)
}

func (s *Integration1CService) CompleteNomenclature(
	ctx context.Context,
	name string,
) ([]integration.NomenclatureItem, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireClient(); err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, webapp.BadRequest("name is required", nil)
	}

	items, err := s.Client.CompleteNomenclature(ctx, name)
	if err != nil {
		return nil, webapp.Internal("1c nomenclature completion failed", map[string]any{
			"error": err.Error(),
		})
	}

	return items, nil
}

func (s *Integration1CService) CompleteCounterparties(
	ctx context.Context,
	name string,
) ([]integration.CounterpartyItem, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	if err := s.requireClient(); err != nil {
		return nil, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, webapp.BadRequest("name is required", nil)
	}

	items, err := s.Client.CompleteCounterparties(ctx, name)
	if err != nil {
		return nil, webapp.Internal("1c counterparty completion failed", map[string]any{
			"error": err.Error(),
		})
	}

	return items, nil
}

func (s *Integration1CService) requireSession() error {
	if s.Session == nil {
		return apperrors.SessionRequired()
	}
	return nil
}

func (s *Integration1CService) requireClient() error {
	if s.Client == nil {
		return webapp.Internal("1c integration is not configured", nil)
	}
	return nil
}
