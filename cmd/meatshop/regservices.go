package main

import (
	"fmt"

	"github.com/dronm/meatshop/internal/config"
	"github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/services"

	"github.com/dronm/webapp"
)

// registerServices registers all availvable services.
// Can add permChecker to a service if it needs some specific
// permission logic.
func registerServices(
	cfg config.Config,
	permChecker webapp.PermissionChecker,
	mainMenuCache services.MainMenuCache,
) (*integration1c.Client, error) {
	services.RegisterMainMenuService(mainMenuCache)
	services.RegisterApplicationRouteService(mainMenuCache)
	services.RegisterObjectHistoryService()

	var oneCClient *integration1c.Client
	if cfg.Integration1C.URL != "" {
		timeout, err := cfg.Integration1C.TimeoutDuration()
		if err != nil {
			return nil, err
		}
		retryDelay, err := cfg.Integration1C.RetryDelayDuration()
		if err != nil {
			return nil, err
		}
		oneCClient, err = integration1c.NewClient(integration1c.Config{
			URL:        cfg.Integration1C.URL,
			Timeout:    timeout,
			MaxRetries: cfg.Integration1C.MaxRetries,
			RetryDelay: retryDelay,
		})
		if err != nil {
			return nil, fmt.Errorf("initialize 1c integration client: %w", err)
		}
	}
	services.RegisterIntegration1CService(oneCClient)
	services.RegisterIntegration1CJobService()

	services.RegisterGeneratedServices()
	services.RegisterCustomerUserService()
	services.RegisterMaxMiniAppService(cfg.MAX)
	services.RegisterMaxNotificationService()
	services.RegisterNotificationTemplateService()
	services.RegisterNotificationTemplateRecipientService()

	services.RegisterProgAboutService()

	services.RegisterUserService(cfg.Session)

	return oneCClient, nil
}
