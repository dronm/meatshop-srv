package services

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/dronm/webapp"
)

func TestNewOrderDatePolicyEarliestDate(t *testing.T) {
	tests := []struct {
		name               string
		today              string
		afterCutoff        bool
		allowHolidayOrders bool
		wantEarliest       string
	}{
		{
			name:         "weekday before cutoff starts next day",
			today:        "2026-09-14", // Monday.
			wantEarliest: "2026-09-15",
		},
		{
			name:         "at or after 15:00 starts in two days",
			today:        "2026-09-14", // Monday.
			afterCutoff:  true,
			wantEarliest: "2026-09-16",
		},
		{
			name:         "ordinary customer on Saturday starts Monday",
			today:        "2026-09-12", // Saturday.
			wantEarliest: "2026-09-14",
		},
		{
			name:               "privileged customer on Saturday may use Sunday",
			today:              "2026-09-12", // Saturday.
			allowHolidayOrders: true,
			wantEarliest:       "2026-09-13",
		},
		{
			name:         "ordinary customer on Sunday starts Monday",
			today:        "2026-09-13", // Sunday.
			wantEarliest: "2026-09-14",
		},
		{
			name:               "privileged customer on Sunday also starts Monday",
			today:              "2026-09-13", // Sunday.
			allowHolidayOrders: true,
			wantEarliest:       "2026-09-14",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := newOrderDatePolicy(
				mustOrderDate(t, test.today),
				test.afterCutoff,
				test.allowHolidayOrders,
				nil,
			)

			if got := policy.earliest.Format(orderDateLayout); got != test.wantEarliest {
				t.Fatalf("earliest = %s, want %s", got, test.wantEarliest)
			}
			if got := policy.latest.Format(orderDateLayout); got != addOrderDays(t, test.today, 7) {
				t.Fatalf("latest = %s, want today + 7 = %s", got, addOrderDays(t, test.today, 7))
			}
		})
	}
}

func TestOrderDatePolicyHolidayCalendar(t *testing.T) {
	t.Run("Sunday is a holiday by default", func(t *testing.T) {
		policy := newOrderDatePolicy(mustOrderDate(t, "2026-09-14"), false, false, nil)
		if !policy.isHoliday(mustOrderDate(t, "2026-09-20")) {
			t.Fatal("Sunday should be a holiday when it has no calendar override")
		}
		if policy.isHoliday(mustOrderDate(t, "2026-09-19")) {
			t.Fatal("Saturday should be a working day when it has no calendar override")
		}
	})

	t.Run("consecutive arbitrary holidays advance earliest date", func(t *testing.T) {
		policy := newOrderDatePolicy(
			mustOrderDate(t, "2026-09-14"),
			false,
			false,
			map[string]bool{
				"2026-09-15": true,
				"2026-09-16": true,
			},
		)
		if got, want := policy.earliest.Format(orderDateLayout), "2026-09-17"; got != want {
			t.Fatalf("earliest = %s, want %s", got, want)
		}
	})

	t.Run("explicit working override makes Sunday available", func(t *testing.T) {
		policy := newOrderDatePolicy(
			mustOrderDate(t, "2026-09-12"), // Saturday.
			false,
			false,
			map[string]bool{"2026-09-13": false},
		)
		if got, want := policy.earliest.Format(orderDateLayout), "2026-09-13"; got != want {
			t.Fatalf("earliest = %s, want %s", got, want)
		}
		if policy.isHoliday(mustOrderDate(t, "2026-09-13")) {
			t.Fatal("Sunday marked as working should not be a holiday")
		}
	})
}

func TestOrderDatePolicyValidateBoundariesAndHolidays(t *testing.T) {
	policy := newOrderDatePolicy(
		mustOrderDate(t, "2026-09-14"), // Monday.
		false,
		false,
		map[string]bool{"2026-09-17": true},
	)

	tests := []struct {
		name     string
		forDate  string
		wantCode string
	}{
		{name: "earliest is accepted", forDate: "2026-09-15"},
		{name: "inclusive today plus seven is accepted", forDate: "2026-09-21"},
		{name: "day before earliest is rejected", forDate: "2026-09-14", wantCode: "order_for_date_too_early"},
		{name: "today plus eight is rejected", forDate: "2026-09-22", wantCode: "order_for_date_too_late"},
		{name: "later default Sunday is rejected", forDate: "2026-09-20", wantCode: "order_for_date_holiday"},
		{name: "later configured holiday is rejected", forDate: "2026-09-17", wantCode: "order_for_date_holiday"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := policy.validate(mustOrderDate(t, test.forDate))
			if test.wantCode == "" {
				if err != nil {
					t.Fatalf("validate() error = %v, want nil", err)
				}
				return
			}
			assertOrderDateHTTPError(t, err, test.wantCode)
		})
	}
}

func TestOrderDatePolicyPrivilegeOnlyBypassesHolidays(t *testing.T) {
	policy := newOrderDatePolicy(
		mustOrderDate(t, "2026-09-14"), // Monday.
		true,
		true,
		map[string]bool{"2026-09-20": true},
	)

	assertOrderDateHTTPError(
		t,
		policy.validate(mustOrderDate(t, "2026-09-15")),
		"order_for_date_too_early",
	)
	if err := policy.validate(mustOrderDate(t, "2026-09-20")); err != nil {
		t.Fatalf("privileged holiday validate() error = %v, want nil", err)
	}
	assertOrderDateHTTPError(
		t,
		policy.validate(mustOrderDate(t, "2026-09-22")),
		"order_for_date_too_late",
	)
}

func TestOrderDatePolicyMaxOrderDateLimits(t *testing.T) {
	tests := []struct {
		name               string
		allowHolidayOrders bool
		wantUnavailable    []string
	}{
		{
			name:            "ordinary customer receives Sundays and configured holidays",
			wantUnavailable: []string{"2026-09-17", "2026-09-20"},
		},
		{
			name:               "privileged customer has no unavailable holiday dates",
			allowHolidayOrders: true,
			wantUnavailable:    []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := newOrderDatePolicy(
				mustOrderDate(t, "2026-09-14"), // Monday.
				false,
				test.allowHolidayOrders,
				map[string]bool{"2026-09-17": true},
			)
			policy.businessTimezone = "Asia/Yekaterinburg"
			limits := policy.maxOrderDateLimits()

			if !limits.Available {
				t.Error("Available = false, want true")
			}
			if got, want := limits.MinimumForDate, "2026-09-15"; got != want {
				t.Errorf("MinimumForDate = %s, want %s", got, want)
			}
			if got, want := limits.MaximumForDate, "2026-09-21"; got != want {
				t.Errorf("MaximumForDate = %s, want %s", got, want)
			}
			if got, want := limits.CutoffTime, "15:00"; got != want {
				t.Errorf("CutoffTime = %s, want %s", got, want)
			}
			if got, want := limits.BusinessTimezone, "Asia/Yekaterinburg"; got != want {
				t.Errorf("BusinessTimezone = %s, want %s", got, want)
			}
			if got := limits.AllowHolidayOrders; got != test.allowHolidayOrders {
				t.Errorf("AllowHolidayOrders = %t, want %t", got, test.allowHolidayOrders)
			}
			if !reflect.DeepEqual(limits.UnavailableForDates, test.wantUnavailable) {
				t.Errorf("UnavailableForDates = %#v, want %#v", limits.UnavailableForDates, test.wantUnavailable)
			}
		})
	}
}

func TestOrderDatePolicyMaxOrderDateLimitsWhenNoDateIsAvailable(t *testing.T) {
	overrides := make(map[string]bool)
	for day := 1; day <= orderMaximumAdvanceDays; day++ {
		overrides[addOrderDays(t, "2026-09-14", day)] = true
	}
	policy := newOrderDatePolicy(
		mustOrderDate(t, "2026-09-14"),
		false,
		false,
		overrides,
	)

	limits := policy.maxOrderDateLimits()
	if limits.Available {
		t.Error("Available = true, want false")
	}
	if limits.MinimumForDate != "" {
		t.Errorf("MinimumForDate = %q, want empty", limits.MinimumForDate)
	}
	if got, want := len(limits.UnavailableForDates), orderMaximumAdvanceDays; got != want {
		t.Errorf("len(UnavailableForDates) = %d, want %d", got, want)
	}
	assertOrderDateHTTPError(
		t,
		policy.validate(mustOrderDate(t, "2026-09-15")),
		"order_for_date_unavailable",
	)
}

func TestSameOrderDateComparesCivilComponents(t *testing.T) {
	utcDate := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
	plusFive := time.FixedZone("UTC+5", 5*60*60)
	localDate := time.Date(2026, time.September, 15, 23, 30, 0, 0, plusFive)

	if !sameOrderDate(utcDate, localDate) {
		t.Fatal("same civil date in different locations should compare equal")
	}
	if sameOrderDate(utcDate, localDate.AddDate(0, 0, 1)) {
		t.Fatal("different civil dates should not compare equal")
	}
}

func assertOrderDateHTTPError(t *testing.T, err error, wantDetailCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("validate() error = nil, want detail code %q", wantDetailCode)
	}

	var httpErr webapp.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("validate() error type = %T, want webapp.HTTPError", err)
	}
	if got := httpErr.StatusCode(); got != http.StatusBadRequest {
		t.Errorf("HTTP status = %d, want %d", got, http.StatusBadRequest)
	}
	details, ok := httpErr.Details().(map[string]any)
	if !ok {
		t.Fatalf("HTTP error details type = %T, want map[string]any", httpErr.Details())
	}
	if got, _ := details["code"].(string); got != wantDetailCode {
		t.Errorf("detail code = %q, want %q", got, wantDetailCode)
	}
}

func mustOrderDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse(orderDateLayout, value)
	if err != nil {
		t.Fatalf("parse test date %q: %v", value, err)
	}
	return date
}

func addOrderDays(t *testing.T, value string, days int) string {
	t.Helper()
	return mustOrderDate(t, value).AddDate(0, 0, days).Format(orderDateLayout)
}
