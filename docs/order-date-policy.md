# Order date policy

The backend applies one order-date policy to admin and MAX order creation. On
updates, it reapplies the policy only when `for_date` changes, or when an admin
changes the order customer. Existing schedules therefore remain editable for
quantity, comment, and status corrections.

## Rules

- Before 15:00 business time, the first candidate is tomorrow.
- At or after 15:00, the first candidate is the day after tomorrow.
- The maximum is business today plus seven calendar days, inclusive.
- Sunday is a holiday unless the calendar explicitly marks it as working.
- Any date can be explicitly marked as a holiday.
- `customers.allow_holiday_orders` bypasses only the holiday restriction. The
  lead-time and maximum-date rules still apply.

The business timezone comes from `public.register_settings`. Configure it with
an IANA/PostgreSQL timezone before relying on the 15:00 cutoff:

```sql
UPDATE public.register_settings
SET business_timezone = '<IANA timezone>'
WHERE id = 1;
```

## Calendar overrides

`public.order_calendar_days` is sparse: dates without rows use the Sunday
default. `id` is the generated CRUD key, while `calendar_date` is the unique
business key. Add a holiday or a working-Sunday override with:

```sql
INSERT INTO public.order_calendar_days (calendar_date, is_holiday, name)
VALUES ('2026-12-31', true, 'New Year holiday')
ON CONFLICT (calendar_date) DO UPDATE
SET is_holiday = EXCLUDED.is_holiday,
    name = EXCLUDED.name;

INSERT INTO public.order_calendar_days (calendar_date, is_holiday, name)
VALUES ('2026-09-20', false, 'Working Sunday')
ON CONFLICT (calendar_date) DO UPDATE
SET is_holiday = EXCLUDED.is_holiday,
    name = EXCLUDED.name;
```

`schema/orderCalendarDay.yaml` generates secured CRUD endpoints at
`/api/order-calendar-days`. Detail, update, and delete routes use the numeric
`id`. Generated JSON clients serialize `calendar_date` as an RFC 3339 value;
PostgreSQL stores only its calendar-date part.

## MAX client contract

`GET /api/max/order-date-limits` returns the current customer-specific minimum,
maximum, unavailable dates, holiday privilege, cutoff, and business timezone.
`available` is false and `minimum_for_date` is empty when the seven-day window
has no valid date.

Date validation failures are HTTP 400 responses. Their details contain one of
these stable business codes:

- `order_for_date_too_early`
- `order_for_date_too_late`
- `order_for_date_holiday`
- `order_for_date_unavailable`
