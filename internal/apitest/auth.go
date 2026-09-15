//go:build integration

package apitest

import (
	"net/http"
	"testing"
)

func TestAuthSmoke(t *testing.T) {
	c := NewClient(t)
	c.Login(t)

	c.DoJSON(
		t,
		http.MethodGet,
		"/api/main-menus/for-user",
		nil,
		http.StatusOK,
	)
}
