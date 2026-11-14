package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
)

type maxCashOrderDetailQuerier struct {
	ds.Querier
}

func (maxCashOrderDetailQuerier) Query(context.Context, string, ...any) (ds.Rows, error) {
	return &orderStatusTestRows{scan: func(dest ...any) error {
		*dest[0].(*int) = 101
		*dest[1].(*int64) = 2
		*dest[2].(*time.Time) = time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
		*dest[5].(*string) = orderStatusProcessing
		*dest[6].(*string) = "В обработке"
		*dest[7].(*int) = 10
		*dest[8].(*string) = "Заказчик"
		*dest[10].(*bool) = true
		*dest[11].(**string) = stringPointer("Иван Иванов")
		*dest[12].(**string) = stringPointer("+79001234567")
		return nil
	}}, nil
}

func TestFetchMaxCashOrderWithoutSalePlaceIsReadOnly(t *testing.T) {
	detail, err := fetchMaxOrderDetail(context.Background(), maxCashOrderDetailQuerier{}, 101, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.CashPayed || detail.CanChange || detail.CustomerSalePlace != nil {
		t.Fatalf("MAX cash order = %#v", detail)
	}
	if detail.CustomerName == nil || *detail.CustomerName != "Иван Иванов" ||
		detail.CustomerPhone == nil || *detail.CustomerPhone != "+79001234567" {
		t.Fatalf("MAX cash order contact = %#v", detail)
	}
	data := orderNotificationDataFromMaxDetail(detail, false)
	if data.SalePlaceName != "" || data.CustomerName != "Заказчик" {
		t.Fatalf("notification data for missing sale place = %#v", data)
	}
}

func TestValidateMaxInitData(t *testing.T) {
	const token = "test-bot-token"
	now := time.Unix(1771409719, 0)
	values := url.Values{}
	values.Set("auth_date", "1771409719")
	values.Set("query_id", "query-1")
	values.Set("user", `{"id":67890,"first_name":"Max","last_name":"User","username":null,"language_code":"ru","photo_url":null}`)
	values.Set("hash", signMaxValues(values, token))

	user, _, err := validateMaxInitData(values.Encode(), token, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("validateMaxInitData() error = %v", err)
	}
	if user.ID != 67890 {
		t.Fatalf("user.ID = %d, want 67890", user.ID)
	}
	if user.Username != nil {
		t.Fatalf("user.Username = %#v, want nil", user.Username)
	}
}

func TestValidateMaxInitDataRejectsChangedData(t *testing.T) {
	const token = "test-bot-token"
	now := time.Unix(1771409719, 0)
	values := url.Values{}
	values.Set("auth_date", "1771409719")
	values.Set("user", `{"id":1,"first_name":"Max"}`)
	values.Set("hash", signMaxValues(values, token))
	values.Set("user", `{"id":2,"first_name":"Max"}`)

	if _, _, err := validateMaxInitData(values.Encode(), token, 24*time.Hour, now); err == nil {
		t.Fatal("validateMaxInitData() expected signature error")
	}
}

func TestInitialMaxUsernames(t *testing.T) {
	tests := []struct {
		name            string
		username        *string
		firstName       string
		wantUsername    *string
		wantAppUsername string
	}{
		{name: "missing", wantAppUsername: defaultMaxAppUsername},
		{name: "blank", username: stringPointer(" \t "), firstName: "  ", wantAppUsername: defaultMaxAppUsername},
		{name: "first name fallback", firstName: " Max User ", wantAppUsername: "Max User"},
		{name: "blank username uses first name", username: stringPointer(" \t "), firstName: " Max User ", wantAppUsername: "Max User"},
		{name: "username takes precedence", username: stringPointer(" max-user "), firstName: "Max User", wantUsername: stringPointer("max-user"), wantAppUsername: "max-user"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			username, appUsername := initialMaxUsernames(test.username, test.firstName)
			if !optionalStringsEqual(username, test.wantUsername) {
				t.Fatalf("initialMaxUsernames() username = %#v, want %#v", username, test.wantUsername)
			}
			if appUsername != test.wantAppUsername {
				t.Fatalf("initialMaxUsernames() app username = %q, want %q", appUsername, test.wantAppUsername)
			}
		})
	}
}

func TestValidateMaxCustomerLookup(t *testing.T) {
	tests := []struct {
		name            string
		input           models.MaxCustomerLookupRequest
		wantINN         string
		wantAppUsername string
		wantError       bool
	}{
		{name: "valid and trimmed", input: models.MaxCustomerLookupRequest{INN: " 1234567890 ", AppUsername: " Иван Иванов "}, wantINN: "1234567890", wantAppUsername: "Иван Иванов"},
		{name: "missing inn", input: models.MaxCustomerLookupRequest{AppUsername: "Иван Иванов"}, wantError: true},
		{name: "blank app username", input: models.MaxCustomerLookupRequest{INN: "1234567890", AppUsername: " \t "}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inn, appUsername, err := validateMaxCustomerLookup(test.input)
			if test.wantError {
				if err == nil {
					t.Fatal("validateMaxCustomerLookup() expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateMaxCustomerLookup() error = %v", err)
			}
			if inn != test.wantINN {
				t.Fatalf("validateMaxCustomerLookup() INN = %q, want %q", inn, test.wantINN)
			}
			if appUsername != test.wantAppUsername {
				t.Fatalf("validateMaxCustomerLookup() app username = %q, want %q", appUsername, test.wantAppUsername)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}

func optionalStringsEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func signMaxValues(values url.Values, token string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMAC.Write([]byte(token))
	signatureMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	_, _ = signatureMAC.Write([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(signatureMAC.Sum(nil))
}
