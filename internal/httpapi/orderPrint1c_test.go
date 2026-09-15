package httpapi

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dronm/meatshop/internal/integration1c"
)

func TestWriteOrder1CPDF(t *testing.T) {
	recorder := httptest.NewRecorder()
	body := []byte("%PDF-1.7\nmock")

	writeOrder1CPDF(recorder, integration1c.BinaryResponse{
		Body:        body,
		ContentType: "application/pdf",
		FileName:    "shipment-print-form.pdf",
	}, "shipments.pdf")

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}
	_, params, err := mime.ParseMediaType(response.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parse content disposition: %v", err)
	}
	if params["filename"] != "shipment-print-form.pdf" {
		t.Fatalf("filename = %q", params["filename"])
	}
	if recorder.Body.String() != string(body) {
		t.Fatalf("body = %q", recorder.Body.Bytes())
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("cache control = %q", response.Header.Get("Cache-Control"))
	}
}
