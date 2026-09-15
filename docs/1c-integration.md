# 1C integration

The backend integrates with the `goCOM1c` HTTP server through the
`internal/integration1c` package.

## Configuration

Add the following section to `config.json`:

```json
"integration_1c": {
	"url": "http://127.0.0.1:3001",
	"timeout": "15s",
	"max_retries": 2,
	"retry_delay": "250ms"
}
```

`url` can be the goCOM1c server base URL, the full `/execute` URL, or the full `/bin-data` URL. The client derives both sibling endpoints (`/execute` and `/bin-data`) from the configured URL.

`max_retries` is the number of retries after the initial request. With the value
`2`, a command can be sent at most three times.

Retries use exponential delays based on `retry_delay`:

- first retry: `250ms`;
- second retry: `500ms`.

The client retries transport/network failures and these HTTP statuses:

- `408 Request Timeout`;
- `425 Too Early`;
- `429 Too Many Requests`;
- `500 Internal Server Error`;
- `502 Bad Gateway`;
- `503 Service Unavailable`;
- `504 Gateway Timeout`.

Normal 4xx responses and successful HTTP responses containing
`{"success": false}` are not retried.

## Creating the client

```go
integrationTimeout, err := cfg.Integration1C.TimeoutDuration()
if err != nil {
	return err
}

retryDelay, err := cfg.Integration1C.RetryDelayDuration()
if err != nil {
	return err
}

oneCClient, err := integration1c.NewClient(integration1c.Config{
	URL:        cfg.Integration1C.URL,
	Timeout:    integrationTimeout,
	MaxRetries: cfg.Integration1C.MaxRetries,
	RetryDelay: retryDelay,
})
if err != nil {
	return err
}
```

## Complete products

```go
products, err := oneCClient.CompleteProducts(ctx, "мяс")
if err != nil {
	return err
}

for _, product := range products {
	fmt.Println(product.ID, product.Name)
}
```

The request sent to 1C is:

```json
{
	"command": "complete_catalogue",
	"params": {
		"catalogue_type": "Номенклатура",
		"name": "мяс"
	}
}
```

The method returns `[]integration1c.NomenclatureItem`.

## Complete counterparties

```go
counterparties, err := oneCClient.CompleteCounterparties(ctx, "Иван")
if err != nil {
	return err
}

for _, counterparty := range counterparties {
	fmt.Println(counterparty.ID, counterparty.Name, counterparty.INN)
}
```

The request sent to 1C is:

```json
{
	"command": "complete_catalogue",
	"params": {
		"catalogue_type": "Контрагенты",
		"name": "Иван"
	}
}
```

The method returns `[]integration1c.CounterpartyItem`.

## Errors

Non-2xx responses are returned as `*integration1c.HTTPError`, which includes the
HTTP status and response body.

A successful HTTP response with `"success": false` is returned as
`*integration1c.CommandError`.

Malformed successful responses are returned as JSON decoding errors.

## HTTP API exposed by Meatshop

The backend publishes two authenticated, permission-protected GET endpoints for frontend autocomplete/search.

### Nomenclature

```http
GET /api/integration-1c/nomenclature?name=мяс
```

Permission:

```text
integration1c.nomenclature.complete
```

Result:

```json
[
	{
		"id": "aaaaaaaa-bbbb-cccc",
		"name": "Мясо замороженное"
	},
	{
		"id": "aaaaaaaa-bbbb-dddd",
		"name": "Мясо охлажденное"
	}
]
```

### Counterparties

```http
GET /api/integration-1c/counterparties?name=Иван
```

Permission:

```text
integration1c.counterparty.complete
```

Result:

```json
[
	{
		"id": "aaaaaaaa-bbbb-cccc",
		"name": "ООО \"Ромашка\"",
		"inn": "1234567890"
	},
	{
		"id": "aaaaaaaa-bbbb-dddd",
		"name": "ИП Иванов",
		"inn": "789012345611"
	}
]
```

Both endpoints require a non-empty `name` query parameter. Internally they execute the same 1C command `complete_catalogue`, using `Номенклатура` or `Контрагенты` as `catalogue_type`.

Migration `000025_integration_1c_permissions` creates both permissions and grants them to the `admin` role.

## Asynchronous order creation

The current Order-to-1C workflow is documented separately because it includes automatic enqueueing on normal Order create/update, version-aware stale-job protection, PostgreSQL retries/journaling, and project-specific result consumption.

See [Order creation and synchronization with 1C](order-1c-creation.md).

## Batch order and shipment actions

The batch endpoints accept the same strict request body:

```json
{
	"order_ids": [101, 102]
}
```

`order_ids` contains local numeric `public.orders.id` values, not 1C UUIDs.
The array must contain between 1 and 1000 unique positive IDs. The backend
validates the complete batch before enqueueing a command or contacting 1C and
preserves the supplied order for the generated PDF sheets.

### Print orders synchronously

The preferred batch endpoint is:

```http
POST /api/order/print-1c
```

The single-order compatibility endpoint remains available:

```http
GET /api/order/{id}/print-1c
```

Both endpoints require permission:

```text
order.print1c
```

Every selected Order must exist and have a non-empty `orders.ref_1c.id`. The
single-order endpoint converts its path ID to a one-element array. Both paths
then send the same command to goCOM1c:

```json
{
	"command": "print_order",
	"params": {
		"order_ids": [101, 102]
	}
}
```

`print_order` is a synchronous binary command sent to `/bin-data`. The HTTP
request waits for 1C to return one combined PDF. Producing a separate sheet for
each Order is the responsibility of the 1C command; Meatshop returns the file
unchanged after verifying that it is a PDF.

The response has no base64 or JSON envelope:

```text
Content-Type: application/pdf
Content-Disposition: attachment; filename="<1C filename or orders.pdf>"
Cache-Control: no-store
```

The legacy GET endpoint uses `order-{id}.pdf` as its fallback filename. All
filenames supplied by 1C are reduced to their basename before being returned.

### Create shipments asynchronously

Shipment creation is explicitly requested with:

```http
POST /api/order/create-shipments-1c
```

Permission:

```text
order.createShipments1c
```

Every selected Order must exist and have a non-empty `orders.ref_1c.id`. The
endpoint does not wait for 1C. It stores one `create_shipments` job and returns
`202 Accepted` with its queue ID and status:

```json
{
	"job_id": 123,
	"status": "queued"
}
```

The worker later posts the following command to goCOM1c `/execute`:

```json
{
	"command": "create_shipments",
	"params": {
		"order_ids": [101, 102]
	}
}
```

An identical active batch reuses its existing queued or processing job. The
correlation ID is calculated from a sorted copy of the IDs, while the original
array order remains unchanged in `params`.

The result consumer supports an optional keyed payload:

```json
{
	"success": true,
	"payload": [
		{
			"order_id": 101,
			"id": "shipment-uuid-101",
			"descr": "Shipment 101"
		},
		{
			"order_id": 102,
			"id": "shipment-uuid-102",
			"descr": "Shipment 102"
		}
	]
}
```

Each valid keyed item updates `orders.shipment_ref_1c` for that local Order.
The current mock returns `"payload": []`; this is accepted as a successful
no-op, but it cannot populate `shipment_ref_1c`.

### Print shipments synchronously

Shipment print forms are requested with:

```http
POST /api/order/print-shipment-1c
```

Permission:

```text
order.printShipment1c
```

Every selected Order must exist and have a non-empty `orders.ref_1c.id`.
`shipment_ref_1c` is not required because the wire contract identifies
shipments by local `order_ids`; this also allows the current empty-payload mock
flow to be printed. Meatshop sends this command to `/bin-data`:

```json
{
	"command": "print_shipment",
	"params": {
		"order_ids": [101, 102]
	}
}
```

Like `print_order`, this is a synchronous request returning one combined PDF.
The fallback filename is `shipments.pdf`.

Both print paths buffer the combined PDF in memory. The 1000-ID validation
ceiling is a safety bound, not a recommended batch size; production batch sizes
and `integration_1c.timeout` should be chosen for the expected PDF volume.

Migration `000028_order_print_1c` grants `order.print1c` to the `admin` role.
Migration `000050_order_shipments_1c` adds the two shipment permissions, grants
them to `admin`, and prevents duplicate active `create_shipments` jobs for the
same correlation ID.
