package integration1c

import (
	"context"
	"fmt"
)

const commandCompleteCatalogue = "complete_catalogue"

type CatalogueType string

const (
	CatalogueNomenclature CatalogueType = "Номенклатура"
	CatalogueCounterparty CatalogueType = "Контрагенты"
)

type CompleteCatalogueParams struct {
	CatalogueType CatalogueType `json:"catalogue_type"`
	Name          string        `json:"name"`
}

type NomenclatureItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CounterpartyItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	INN  string `json:"inn"`
}

func (c *Client) CompleteNomenclature(ctx context.Context, name string) ([]NomenclatureItem, error) {
	return completeCatalogue[NomenclatureItem](ctx, c, CatalogueNomenclature, name)
}

// CompleteProducts is kept as a compatibility alias. New code should use CompleteNomenclature.
func (c *Client) CompleteProducts(ctx context.Context, name string) ([]NomenclatureItem, error) {
	return c.CompleteNomenclature(ctx, name)
}

func (c *Client) CompleteCounterparties(ctx context.Context, name string) ([]CounterpartyItem, error) {
	return completeCatalogue[CounterpartyItem](ctx, c, CatalogueCounterparty, name)
}

func completeCatalogue[T any](ctx context.Context, client *Client, catalogueType CatalogueType, name string) ([]T, error) {
	if !catalogueType.Valid() {
		return nil, fmt.Errorf("unsupported 1c catalogue type %q", catalogueType)
	}

	return execute[T](ctx, client, commandCompleteCatalogue, CompleteCatalogueParams{
		CatalogueType: catalogueType,
		Name:          name,
	})
}

func (t CatalogueType) Valid() bool {
	switch t {
	case CatalogueNomenclature, CatalogueCounterparty:
		return true
	default:
		return false
	}
}
