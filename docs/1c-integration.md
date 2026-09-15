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


## Synchronous order print form PDF

An authenticated admin can download the 1C print form for an order that already has `orders.ref_1c`:

```http
GET /api/order/{id}/print-1c
```

Permission:

```text
order.print1c
```

The endpoint uses the local Meatshop order ID. The backend loads `orders.ref_1c.id` and sends the following provisional command to goCOM1c:

```json
{
  "command": "order_print_form",
  "params": {
    "order_id": "document-uuid"
  }
}
```

Unlike normal JSON commands, this command is sent to the goCOM1c `/bin-data` endpoint. goCOM1c streams the temporary file returned by 1C as binary data. Meatshop preserves those bytes, verifies that the response is a PDF, and returns it directly to the client with:

```text
Content-Type: application/pdf
Content-Disposition: attachment; filename="order-123.pdf"
Cache-Control: no-store
```

There is no base64 or JSON envelope on the Meatshop response. The provisional 1C command name and parameter structure are isolated in `internal/integration1c/order_print.go` so they can be changed when the final 1C procedure contract is known.

Migration `000028_order_print_1c` adds the `order.print1c` permission for the `admin` role.
