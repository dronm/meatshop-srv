# Order creation and automatic corrections in 1C

## User workflow

- Saving a new order in the admin application or MAX saves it locally. It does not create a 1C job and does not require the customer/products to have 1C references at this stage.
- Select orders in either the Orders or Order Lines collection and press **Создать заказ**. Multiple selected lines belonging to the same order are submitted once.
- The action sends complete orders, including all their lines, rather than only the selected lines.
- Correcting an order after its first explicit send continues to synchronize automatically through the existing `create_order` command. This also applies while its first send is queued or processing.
- An order that has never been sent remains a local draft when corrected. Use the button for its first transmission.
- The existing shipment and print commands retain their existing behavior and reference requirements.

## Deployment

1. Update the external 1C processing object's module using `ObjectModule(1).bsl`. Keep the configuration-specific pricing, warehouse, organization, and printing helpers supplied in the original module.
2. Apply `migrations/000063_order_create_batch_1c.up.sql` after the earlier migrations, including order-lines migration 62.
3. Deploy and restart the backend, including its embedded integration worker, then deploy the rebuilt admin frontend.
4. For local testing, update the Mockoon project and restart it.

The bulk endpoint reuses the existing `order.create1c` permission. No new permission assignment is needed for users who already have it. Existing queued `create_order` jobs remain valid. No files need to be deleted, and generated files are unchanged.

## HTTP API

New bulk action:

```http
POST /api/order/create-orders-1c
Content-Type: application/json

{"order_ids":[101,102]}
```

It returns HTTP 202 with the existing job envelope:

```json
{"job_id":123,"status":"queued"}
```

The request accepts 1–1000 distinct positive local order IDs. Existing orders are locked in ascending ID order. Missing orders, missing customer/product 1C references, and empty orders reject the request before a job is committed.

The old single-order action remains available:

```http
POST /api/order/101/create-1c
```

## Gateway commands

Bulk creation uses `POST /execute` with this envelope:

```json
{
	"command":"create_orders",
	"params":{
		"orders":[
			{
				"order_id":101,
				"order_version":2,
				"customer_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"products":[
					{"order_item_id":201,"id":"11111111-aaaa-bbbb-cccc-222222222222","quant":2}
				]
			}
		]
	}
}
```

The successful response is an array:

```json
{
	"success":true,
	"payload":[
		{
			"order_id":101,
			"order_version":2,
			"id":"00000000-0000-4000-8000-000000000101",
			"descr":"Заказ покупателя 000101",
			"number_1c":"000101",
			"items":[
				{
					"order_item_id":201,
					"price":"120.000000",
					"amount":"240.00",
					"vat_percent":"20.00",
					"vat_amount":"40.00",
					"use_marking":true
				}
			]
		}
	]
}
```

`create_order` retains its existing single-order request and object-shaped payload. It is used for automatic corrections and remains accepted by the 1C module and mock.

## Queue, retries, and corrections

- A bulk job stores immutable snapshots of the order versions, customer references, products, quantities, and local line IDs.
- Its correlation ID hashes the canonical snapshots. Repeating an identical active request returns the same job. Overlapping active creation requests are rejected instead of sending the same orders concurrently.
- Automatic corrections use versioned single-order jobs. If their first batch is still active, `depends_on_job_ids` makes them wait until that batch finishes. The queue worker checks these dependencies when claiming work, without consuming retry attempts while waiting.
- The first explicitly requested batch is sent even if an order is corrected meanwhile. The correction follows it; obsolete intermediate single-order versions are skipped.
- Responses retain the existing current-version, latest-job, customer, and line-snapshot checks. A response for an earlier local version cannot overwrite the correction. The latest-job check covers both single and bulk jobs.
- The entire bulk response is checked for missing, repeated, or unexpected order IDs, incorrect versions, missing invoice IDs, incorrect line identities, malformed decimal strings, and missing marking flags before applying any returned values.
- Last-calculated monetary values are cleared when a synchronization is requested or an unsent draft is corrected, preserving the previous invalidation behavior.

## 1C implementation

`ВыполнитьСозданиеЗаказа` and `ВыполнитьСозданиеЗаказов` share `ПодготовитьДанныеСозданияЗаказа` and `СоздатьСчетПоПодготовленномуЗаказу`.

The bulk method validates the input and rejects duplicate order IDs, acquires shared order locks in ascending order, then creates or updates all invoices in one managed transaction. A failure rolls back the entire batch. Existing `order_id`, `order_version`, and `order_item_id` idempotency checks are retained. An equal version returns the saved calculation; a newer version updates the existing invoice.

The original configuration-specific helpers and all older commands are retained. The module was reviewed structurally, but a 1C runtime is not available in this environment; verify it in your actual configuration before production use.

## Validation

- Backend `go test ./...` passes using the upstream dependency sources in an isolated verification checkout.
- Frontend TypeScript checking and the Vite production build pass using the upstream Vue library sources.
- Both collections were exercised with repeated order IDs and empty 1C references; they sent one ID per order to the new endpoint.
- Mockoon HTTP tests cover batch creation, repeated products, zero quantities, decimal strings, marking flags, legacy single creation, automatic single-order corrections using the same invoice reference, shipments, and both PDF commands.
- PostgreSQL checks exercise the bulk index migration in both directions, correction eligibility, and queue dependencies.
