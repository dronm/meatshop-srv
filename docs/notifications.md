# MAX notification templates and Order events

The application uses `public.max_out_messages` as the durable outgoing MAX queue. Notification templates and recipient configuration only decide **what** to send and **who** should receive it; `cmd/max` remains responsible for delivery and retry handling.

## Configuration tables

### `notification_templates`

- `id` — primary key.
- `code` — stable unique template code.
- `event` — domain event used to select the template.
- `body_template` — Go `text/template` body.
- `is_active` — disabled templates are ignored.

Migration `000038_notification_templates` creates two default events:

- `order.submitted` — a customer submits a new Order or resubmits an editable Order. The same template/recipient configuration is used in both cases; `Resubmitted` distinguishes the message text.
- `order.changed` — an employee changes Order status or changes a confirmed quantity so that it differs from `quant_required`.

### `notification_template_recipients`

This table configures responsible internal employees for a template:

- `template_id` -> `notification_templates.id`.
- `user_id` -> `users.id`.

At enqueue time the backend resolves:

`notification_template_recipients.user_id -> users.max_user_id -> max_users.id -> max_users.max_user_id`

Only active MAX accounts are queued. A responsible employee therefore needs a MAX account binding in the User edit form.

`order.changed` does not use `notification_template_recipients`; its recipient is the `orders.customer_user_id` MAX user that originally submitted the Order.

## Order events

### Customer creates a new Order

`MaxMiniAppService.CreateOrder()` saves the Order/items, queues 1C synchronization, fetches the completed Order detail and enqueues the `order.submitted` notification for all configured responsible employees. All inserts use the same PostgreSQL transaction.

### Customer resubmits an Order

`MaxMiniAppService.UpdateOrder()` performs the same notification flow with `Resubmitted=true`. Responsible employees are configured only once on the `order_submitted` template.

### Employee changes status or confirmed quantity

`OrderService.Update()` locks the current Order and item quantities before updating. After the new document is stored it sends `order.changed` to the original customer MAX user when either:

- `status_id` differs from the old status; or
- an item was actually changed and its resulting `quant` differs from `quant_required`.

An old quantity mismatch that is left unchanged does not generate another notification on an unrelated edit.

## Available template fields

Order templates can use these Go `text/template` fields:

- `{{.OrderID}}`
- `{{.Number1C}}`
- `{{.ForDate}}`
- `{{.CustomerName}}`
- `{{.SalePlaceName}}`
- `{{.StatusName}}`
- `{{.CommentCustomer}}`
- `{{.StatusChanged}}`
- `{{.ChangedItems}}` — newline-separated `requested -> confirmed` lines for changed items.
- `{{.Resubmitted}}` — true for a customer resend/update.

Example:

```text
{{if .Resubmitted}}Заказ №{{.OrderID}} изменён клиентом{{else}}Новый заказ №{{.OrderID}}{{end}}
Клиент: {{.CustomerName}}
Дата: {{.ForDate}}
```

## Delivery

Rendered messages are inserted into `public.max_out_messages` with metadata containing the event, Order ID and template ID/code. PostgreSQL `NOTIFY max_out_messages` wakes the existing sender after commit. The sender handles rate limiting, retries and permanent failures.
