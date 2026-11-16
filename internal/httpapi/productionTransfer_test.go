package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dronm/meatshop/internal/models"
)

func TestProductionDocumentBinderKeepsPathIdentityAndEditableMembership(t *testing.T) {
	request := httptest.NewRequest(http.MethodPatch, "/api/production-transfers/17", strings.NewReader(`{"version":4,"document_date":"2026-11-13","for_date":"2026-11-14","comment":null,"order_ids":[],"request_id":"request-1","order_versions":{"21":3}}`))
	request.SetPathValue("id", "17")
	bound, err := productionDocumentBinder(true)(request)
	if err != nil {
		t.Fatal(err)
	}
	input, ok := bound.(models.ProductionTransferUpdateRequest)
	if !ok || input.ID != 17 || input.Document.Version != 4 || len(input.Document.OrderIDs) != 0 || input.Document.OrderVersions[21] != 3 {
		t.Fatalf("incorrect update binding: %#v", bound)
	}
}

func TestProductionDocumentBinderRejectsUntrustedFieldsAndTrailingJSON(t *testing.T) {
	for _, body := range []string{
		`{"order_ids":[1],"status_id":3}`,
		`{"order_ids":[1],"id":99}`,
		`{"order_ids":[1]} {}`,
		`{"order_ids":[1.5]}`,
		strings.Repeat(" ", 256<<10+1),
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/production-transfers", strings.NewReader(body))
		if _, err := productionDocumentBinder(false)(request); err == nil {
			t.Fatalf("invalid request accepted (%d bytes)", len(body))
		}
	}
}
