# Order document aggregate and products register

## Order API aggregate

`Order` is handled as one transactional aggregate: the header in `orders` and all detail rows in `order_items` are created or updated together from one JSON request.

Endpoints:

- `POST /api/order` creates the complete order and returns the saved header plus generated item ids.
- `GET /api/order/{id}` returns an `OrderDetail` projection containing the
  editable foreign key values, their reference objects, nullable 1C/comment
  fields, and `items` with product and measure-unit references.
- `PUT /api/order/{id}` replaces the editable header fields and synchronizes the complete `items` array. Existing rows keep their ids, omitted rows are deleted, and rows with `id = 0`/no id are inserted.
- `DELETE /api/order/{id}` deletes the order and removes its register actions in the same transaction.
- `GET /api/order` remains the generated collection endpoint based on `orders_list`.
- `GET /api/order/lines` lists one row per item from `order_lines_list`, with collection filtering, sorting, pagination, and the existing `order.list` permission. It returns `id` (the parent order ID), `shipment_ref_1c`, `customer`, `customer_sale_place`, `product`, `quant`, `price`, `amount`, `vat_amount`, `use_marking`, `item_id`, `line_num`, and `ref_1c`. The `id` may repeat across lines; `item_id` uniquely identifies each displayed row. Sort by `id`, `line_num`, and `item_id` for stable pagination.

Create and update accept the writable `OrderDocument` command model and return
the saved `OrderDetail` projection. This keeps client-supplied reference labels
out of persistence commands while giving edit forms everything required to
initialize reference inputs.

`version` implements optimistic concurrency. A client must send the version returned by detail/create/update when calling `PUT`. The service increments it after every successful complete update.

Standalone `/api/order-items` routes are not exposed. Order items are owned by their Order aggregate.

## Products accumulation register

The authoritative action table is `ra_products`.

Dimensions:

- `customer_id`
- `customer_sale_place_id`
- `product_id`
- `measure_unit_id`

Facts:

- `quant_required`
- `quant`

Each Order item writes one action with recorder type `Order`. `orders.for_date` is converted to local business midnight with `register_date_start()`.

`rg_products` stores monthly net aggregate values. `rg_products_current` stores a fast aggregate over all currently posted register actions.

Register actions are immutable. Editing an Order removes all actions for that recorder and inserts replacement actions from the final saved Order state. Insert/delete triggers incrementally maintain both aggregate tables.

Available SQL functions:

- `ra_products_add_act(...)`
- `ra_products_remove_acts(recorder_type, recorder_id)`
- `rg_products_apply_delta(...)`
- `rg_products_rebuild()`
- `rg_products_balance(customer_ids, customer_sale_place_ids, product_ids, measure_unit_ids)`
- `rg_products_balance(effective_at, customer_ids, customer_sale_place_ids, product_ids, measure_unit_ids)`

The exact-time balance function uses completed monthly values from `rg_products` plus raw `ra_products` actions for the requested month. This keeps historical reads fast without storing cumulative month-end snapshots that would need cascading recalculation after backdated edits.

## Common register infrastructure

Migration `000018_register_common` creates shared register helpers:

- singleton `register_settings`
- configurable `business_timezone`
- timezone validation trigger
- `register_business_timezone()`
- `register_date_start(date)`
- `register_month_start(timestamptz)`
- `register_month_start_at(timestamptz)`

The default timezone is `UTC`. Set `register_settings.business_timezone` to the application's actual business timezone before production data is posted. If it is changed after register actions exist, run `SELECT public.rg_products_rebuild();`.

## Posting assumption

At this stage every saved Order posts to the products register regardless of `status_id`. If cancelled/draft statuses must not participate in product totals, add the posting rule to `rebuildProductRegisterActions()` once the status semantics are finalized.
