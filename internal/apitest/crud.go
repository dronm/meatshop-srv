//go:build integration

package apitest

import (
	"fmt"
	"net/http"
	"testing"
)

type CRUDCase struct {
	Name       string
	BasePath   string
	Key        string
	CreateBody any
	UpdateBody any

	CheckCreated func(t *testing.T, detail map[string]any)
	CheckUpdated func(t *testing.T, detail map[string]any)
}

type GeneratedKeyCRUDCase struct {
	Name        string
	BasePath    string
	CreateBody  any
	UpdateBody  any
	KeyFromBody func(t *testing.T, body map[string]any) string

	CheckCreated func(t *testing.T, detail map[string]any)
	CheckUpdated func(t *testing.T, detail map[string]any)
}

func RunCRUD(t *testing.T, c *Client, tc CRUDCase) {
	t.Helper()

	detailPath := fmt.Sprintf("%s/%s", tc.BasePath, tc.Key)

	c.DeleteIgnore(t, detailPath)

	t.Cleanup(func() {
		c.DeleteIgnore(t, detailPath)
	})

	c.DoJSON(t, http.MethodPost, tc.BasePath, tc.CreateBody, http.StatusCreated)

	created := c.DoJSON(t, http.MethodGet, detailPath, nil, http.StatusOK)
	if tc.CheckCreated != nil {
		tc.CheckCreated(t, created)
	}

	c.DoJSON(t, http.MethodGet, tc.BasePath, nil, http.StatusOK)

	c.DoJSON(t, http.MethodPatch, detailPath, tc.UpdateBody, http.StatusOK)

	updated := c.DoJSON(t, http.MethodGet, detailPath, nil, http.StatusOK)
	if tc.CheckUpdated != nil {
		tc.CheckUpdated(t, updated)
	}

	c.DoJSON(t, http.MethodDelete, detailPath, nil, http.StatusOK)

	c.DoJSON(t, http.MethodGet, detailPath, nil, http.StatusNotFound)
	c.DoJSON(t, http.MethodDelete, detailPath, nil, http.StatusNotFound)
}

func RunGeneratedKeyCRUD(t *testing.T, c *Client, tc GeneratedKeyCRUDCase) string {
	t.Helper()

	createdBody := c.DoJSON(t, http.MethodPost, tc.BasePath, tc.CreateBody, http.StatusCreated)
	key := tc.KeyFromBody(t, createdBody)
	if key == "" {
		t.Fatalf("%s: created response did not contain a usable key", tc.Name)
	}

	detailPath := fmt.Sprintf("%s/%s", tc.BasePath, key)
	t.Cleanup(func() {
		c.DeleteIgnore(t, detailPath)
	})

	created := c.DoJSON(t, http.MethodGet, detailPath, nil, http.StatusOK)
	if tc.CheckCreated != nil {
		tc.CheckCreated(t, created)
	}

	c.DoJSON(t, http.MethodGet, tc.BasePath, nil, http.StatusOK)

	c.DoJSON(t, http.MethodPatch, detailPath, tc.UpdateBody, http.StatusOK)

	updated := c.DoJSON(t, http.MethodGet, detailPath, nil, http.StatusOK)
	if tc.CheckUpdated != nil {
		tc.CheckUpdated(t, updated)
	}

	c.DoJSON(t, http.MethodDelete, detailPath, nil, http.StatusOK)

	c.DoJSON(t, http.MethodGet, detailPath, nil, http.StatusNotFound)
	c.DoJSON(t, http.MethodDelete, detailPath, nil, http.StatusNotFound)

	return key
}
