# Order lines: columns, search, and default page

## Deployment

1. Apply pending migrations through `000065_order_lines_list_status` before deploying the updated backend and frontend. Migration `000062` appends `orders.for_date`; migration `000065` appends the parent order's status reference.
2. Replace the project files with the updated archives, rebuild the backend and frontend, and restart the application.

There are no files to delete. Generated files and dependency manifests are unchanged.

## Behavior

- The visible “Строка” column is removed. Internal line numbers and item IDs remain available for deterministic ordering and row identity.
- “Дата заказа” displays `orders.for_date`. The library provides exact calendar-day and date-range search.
- Customer, sale place, and product search use the existing reference inputs and exact equality on `<field>->'keys'->>'id'`.
- Sale-place search can select places across customers. The order form still requires its customer and limits places to that customer.
- Order number and the displayed 1C order/shipment captions are searchable. The 1C fields have the separate `{id, descr}` format; their captions use text search.
- Quantity, price, amount, VAT, and marking are excluded from search.
- Loading the home page or signing in opens `orderLines` (`/order-lines`).
- Existing order-group borders and edit/copy navigation are retained. The shared commands also include on-demand creation in 1C; see `order-1c-creation.md`.
- Order lines share the Orders status colors. Marked-row highlighting and the current-cell outline take priority over status backgrounds.

The endpoint continues using `modelbind` and `webapp.FetchCollectionModel`. A small parameter conversion supplies numeric picker IDs as text for PostgreSQL's `->>` expressions; pgx otherwise rejects numeric values for text parameters.

## Modified files

Backend:

- `internal/models/orderLineList.go`
- `internal/services/orderLinesList.go`
- `internal/services/orderLinesList_test.go` (new)
- `migrations/000062_order_lines_list_date.up.sql` (new)
- `migrations/000062_order_lines_list_date.down.sql` (new)
- `migrations/000065_order_lines_list_status.up.sql` (new)
- `migrations/000065_order_lines_list_status.down.sql` (new)
- `migrations/sql/order_lines_list.sql`
- `docs/order-lines-search.md` (new)

Frontend:

- `src/collections/orderLineList.ts`
- `src/references/customerReferences.ts`
- `src/schemas/orderLineList.ts`
- `src/types/orderLineList.ts`
- `src/locales/ru.json`
- `src/views/Main.vue`
- `src/views/Login.vue`
- `src/collections/order.ts`
- `src/utils/orderStatus.ts` (new)

## Validation

- Frontend TypeScript check and Vite production build passed in an isolated checkout using the upstream sources of vue-collection-lib 0.1.14 and vue-business-app 0.1.8. The original dependency declarations were preserved in the delivered archive.
- Backend model, service, and HTTP API tests passed with Go 1.25 and local copies of the upstream dependency sources; the original local Go replacement paths were preserved in the delivered archive.
- The actual collection configuration generated exact reference-key filters and whole-day date boundaries.
- PostgreSQL execution in PGlite verified the migration in both directions, duplicate-description reference matching, date filtering, and per-line identity. No production database was used.
## Status backgrounds

The order-lines projection includes a nullable `status` reference, matching
the Orders collection. Apply migration `000065_order_lines_list_status`
before deploying the updated backend. The frontend uses this field for the
shared status colors while preserving marked-row/current-cell highlights
and order separator borders.

