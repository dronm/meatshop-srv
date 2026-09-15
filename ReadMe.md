# Meatshop backend

Meatshop is the Go/PostgreSQL backend for the ordering application. It provides the admin API, MAX mini-app services, product/order registers, and integration with 1C through a PostgreSQL-backed asynchronous worker.

## Documentation

- [Order creation and synchronization with 1C](docs/order-1c-creation.md) — detailed trace from Order save through the queue/worker to `orders.ref_1c`.
- [1C integration](docs/1c-integration.md) — general 1C integration, autocomplete and print-form API notes.
- [Order document and product register](docs/order-document-register.md) — Order aggregate and register behavior.
- [Order date policy](docs/order-date-policy.md) — cutoff, holiday calendar, customer privilege, and MAX client limits.
- [MAX bot](docs/max-bot.md) — MAX bot integration and message processing.
- [MAX notifications](docs/notifications.md) — templates, responsible recipients, Order events and delivery flow.

## Main backend areas

- `cmd/meatshop` — HTTP server startup and embedded worker wiring.
- `internal/httpapi` — HTTP routes and binders.
- `internal/services` — application/domain services.
- `internal/integration1c` — synchronous 1C client contracts and command types.
- `internal/integration1cworker` — durable asynchronous 1C queue, worker and result consumer.
- `internal/maxbot` — MAX bot transport.
- `internal/models` — API/domain models.
- `migrations` — PostgreSQL schema, functions and permissions.
- `schema` — code-generation schemas.
