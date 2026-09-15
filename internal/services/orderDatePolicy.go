package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

const (
	orderDateLayout         = "2006-01-02"
	orderCutoffTime         = "15:00"
	orderMaximumAdvanceDays = 7
)

type orderDatePolicy struct {
	firstCandidate     time.Time
	earliest           time.Time
	latest             time.Time
	allowHolidayOrders bool
	businessTimezone   string
	calendarOverrides  map[string]bool
}

func loadCustomerOrderDatePolicy(
	ctx context.Context,
	db ds.Querier,
	customerID int,
	now time.Time,
) (*orderDatePolicy, error) {
	var (
		allowHolidayOrders bool
		businessTimezone   string
		businessToday      time.Time
		afterCutoff        bool
	)
	if err := db.QueryRow(ctx, `
		SELECT
			c.allow_holiday_orders,
			public.register_business_timezone(),
			timezone(public.register_business_timezone(), $2::timestamptz)::date,
			timezone(public.register_business_timezone(), $2::timestamptz)::time >= $3::time
		FROM public.customers AS c
		WHERE c.id = $1
	`, customerID, now, orderCutoffTime).Scan(
		&allowHolidayOrders,
		&businessTimezone,
		&businessToday,
		&afterCutoff,
	); err != nil {
		if errors.Is(err, ds.ErrNoRows) {
			return nil, webapp.BadRequest("customer is invalid", map[string]any{
				"code":        "order_customer_invalid",
				"customer_id": customerID,
			})
		}
		return nil, fmt.Errorf("load customer order date settings: %w", err)
	}

	today := normalizeOrderDate(businessToday)
	latest := today.AddDate(0, 0, orderMaximumAdvanceDays)
	overrides, err := loadOrderCalendarOverrides(
		ctx,
		db,
		today.AddDate(0, 0, 1),
		latest,
	)
	if err != nil {
		return nil, err
	}

	policy := newOrderDatePolicy(today, afterCutoff, allowHolidayOrders, overrides)
	policy.businessTimezone = businessTimezone
	return policy, nil
}

func loadOrderCalendarOverrides(
	ctx context.Context,
	db ds.Querier,
	from time.Time,
	through time.Time,
) (map[string]bool, error) {
	rows, err := db.Query(ctx, `
		SELECT calendar_date, is_holiday
		FROM public.order_calendar_days
		WHERE calendar_date BETWEEN $1::date AND $2::date
		ORDER BY calendar_date
	`, from, through)
	if err != nil {
		return nil, fmt.Errorf("load order calendar: %w", err)
	}
	defer rows.Close()

	overrides := make(map[string]bool)
	for rows.Next() {
		var (
			date      time.Time
			isHoliday bool
		)
		if err := rows.Scan(&date, &isHoliday); err != nil {
			return nil, fmt.Errorf("scan order calendar day: %w", err)
		}
		overrides[normalizeOrderDate(date).Format(orderDateLayout)] = isHoliday
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read order calendar: %w", err)
	}
	return overrides, nil
}

func newOrderDatePolicy(
	today time.Time,
	afterCutoff bool,
	allowHolidayOrders bool,
	calendarOverrides map[string]bool,
) *orderDatePolicy {
	today = normalizeOrderDate(today)
	leadDays := 1
	if afterCutoff {
		leadDays = 2
	}
	policy := &orderDatePolicy{
		firstCandidate:     today.AddDate(0, 0, leadDays),
		earliest:           today.AddDate(0, 0, leadDays),
		latest:             today.AddDate(0, 0, orderMaximumAdvanceDays),
		allowHolidayOrders: allowHolidayOrders,
		calendarOverrides:  calendarOverrides,
	}
	if policy.calendarOverrides == nil {
		policy.calendarOverrides = make(map[string]bool)
	}
	if !policy.allowHolidayOrders {
		for policy.isHoliday(policy.earliest) {
			policy.earliest = policy.earliest.AddDate(0, 0, 1)
		}
	}
	return policy
}

func (p *orderDatePolicy) validate(forDate time.Time) error {
	date := normalizeOrderDate(forDate)
	details := p.errorDetails(date)

	if p.earliest.After(p.latest) {
		details["code"] = "order_for_date_unavailable"
		return webapp.BadRequest("no order date is available in the allowed range", details)
	}
	if date.Before(p.earliest) {
		details["code"] = "order_for_date_too_early"
		return webapp.BadRequest("order date is earlier than allowed", details)
	}
	if date.After(p.latest) {
		details["code"] = "order_for_date_too_late"
		return webapp.BadRequest("order date is later than allowed", details)
	}
	if !p.allowHolidayOrders && p.isHoliday(date) {
		details["code"] = "order_for_date_holiday"
		return webapp.BadRequest("orders cannot be placed for a holiday", details)
	}
	return nil
}

func (p *orderDatePolicy) maxOrderDateLimits() *models.MaxOrderDateLimits {
	available := !p.earliest.After(p.latest)
	result := &models.MaxOrderDateLimits{
		Available:           available,
		MinimumForDate:      p.earliest.Format(orderDateLayout),
		MaximumForDate:      p.latest.Format(orderDateLayout),
		UnavailableForDates: make([]string, 0),
		CutoffTime:          orderCutoffTime,
		BusinessTimezone:    p.businessTimezone,
		AllowHolidayOrders:  p.allowHolidayOrders,
	}
	if !available {
		result.MinimumForDate = ""
	}
	if p.allowHolidayOrders {
		return result
	}
	for date := p.firstCandidate; !date.After(p.latest); date = date.AddDate(0, 0, 1) {
		if p.isHoliday(date) {
			result.UnavailableForDates = append(result.UnavailableForDates, date.Format(orderDateLayout))
		}
	}
	return result
}

func (p *orderDatePolicy) isHoliday(date time.Time) bool {
	date = normalizeOrderDate(date)
	if isHoliday, ok := p.calendarOverrides[date.Format(orderDateLayout)]; ok {
		return isHoliday
	}
	return date.Weekday() == time.Sunday
}

func (p *orderDatePolicy) errorDetails(forDate time.Time) map[string]any {
	return map[string]any{
		"for_date":          forDate.Format(orderDateLayout),
		"earliest_for_date": p.earliest.Format(orderDateLayout),
		"latest_for_date":   p.latest.Format(orderDateLayout),
		"cutoff_time":       orderCutoffTime,
		"business_timezone": p.businessTimezone,
		"available":         !p.earliest.After(p.latest),
	}
}

func normalizeOrderDate(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func sameOrderDate(left time.Time, right time.Time) bool {
	return normalizeOrderDate(left).Equal(normalizeOrderDate(right))
}
