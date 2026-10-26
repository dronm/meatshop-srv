# Batch order status changes

`POST /api/order/change-status` uses the existing authenticated session and
`order.update` permission. Both the Orders collection and the Order lines
collection send order IDs to this endpoint. Selecting multiple item rows from
one order changes that order once.

Request:

```json
{
	"order_ids": [101, 102],
	"status_id": 3
}
```

Response:

```json
{
	"rows_affected": 2
}
```

The request requires a positive, existing status and between 1 and 1,000
positive order IDs. Duplicate IDs are deduplicated. If a selected order no
longer exists, the whole request fails. Status changes and their notification
queue records commit in one transaction.

An order already at the requested status stays unchanged and does not count
toward `rows_affected`. Changed orders increment their optimistic concurrency
version, so an edit form opened before the batch cannot overwrite the new
status without reloading. Existing customer MAX notifications and automatic
1C correction synchronization are preserved. Item quantities and product
register actions are not rewritten.

One `Order.Update` event refreshes other connected collections after a changed
batch commits. A batch that changes no statuses emits no event or MAX messages.

No database migration or code generation is required.
