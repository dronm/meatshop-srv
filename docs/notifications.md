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
- `order.changed` — an employee changes Order status, changes a confirmed quantity so that it differs from `quant_required`, or removes an order line.

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

`MaxMiniAppService.CreateOrder()` saves the Order/items, fetches the completed Order detail and enqueues the `order.submitted` notification for all configured responsible employees. Initial 1C creation is requested separately through the collection command. All inserts use the same PostgreSQL transaction.

### Customer resubmits an Order

`MaxMiniAppService.UpdateOrder()` performs the same notification flow with `Resubmitted=true`. Responsible employees are configured only once on the `order_submitted` template.

### Employee changes status, confirmed quantity, or removes a line

`OrderService.Update()` locks the current Order and captures its line IDs, product names, original positions, and quantities before updating. After the new document is stored it sends `order.changed` to the original customer MAX user when any of these conditions holds:

- `status_id` differs from the old status; or
- an item was actually changed and its resulting `quant` differs from `quant_required`; or
- a previously saved line is absent from the updated order, including lines with equal or zero confirmed quantity.

An old quantity mismatch that is left unchanged does not generate another notification on an unrelated edit.

Removed lines are detected by the local line ID, so two lines for the same product are distinguished. Their descriptions and quantities come from the snapshot taken before deletion. Removal details are included in `ChangedItems`, preserving compatibility with existing notification templates, for example:

```text
• Мясо: позиция удалена (заказано: 5; подтверждено: 5)
```

Quantity changes and removed lines in the same save are combined in one notification per active template. A later unchanged save does not repeat a removal. No migration or template replacement is required.

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
- `{{.ChangedItems}}` — newline-separated quantity changes and descriptions of removed lines.
- `{{.Resubmitted}}` — true for a customer resend/update.

Example:

```text
{{if .Resubmitted}}Заказ №{{.OrderID}} изменён клиентом{{else}}Новый заказ №{{.OrderID}}{{end}}
Клиент: {{.CustomerName}}
Дата: {{.ForDate}}
```

## Delivery

Rendered messages are inserted into `public.max_out_messages` with metadata containing the event, Order ID and template ID/code. PostgreSQL `NOTIFY max_out_messages` wakes the existing sender after commit. The sender handles rate limiting, retries and permanent failures.
