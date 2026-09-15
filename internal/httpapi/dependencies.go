package httpapi

import (
	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/integration1c"
)

type Dependencies struct {
	DB                  ds.Provider
	Integration1CClient *integration1c.Client
}
