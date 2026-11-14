# Cash orders and buyer contact details

The order header has `cash_payed` (boolean, default `false`), `customer_name` and `customer_phone` (nullable text). Contacts are optional even for cash orders. Whitespace-only contacts are saved as `NULL`. The selected catalogue customer remains mandatory and determines the 1C counterparty and pricing.

In the admin order form, enabling **Наличный расчет** shows **Имя покупателя** and **Телефон покупателя**, hides the sale-place selector, and clears the sale place. The backend also clears a supplied place for cash orders. Disabling cash mode shows an empty sale-place selector and requires a real place before saving. Contacts are kept on the order; copying an admin order preserves them for review.

Order and order-line collections expose all three fields. No fictitious sale place is created. Product register movements and balances support a missing sale-place dimension with null-aware uniqueness and cleanup.

## Installation

1. Apply both new migrations in order: `000076_cash_order_contacts` and `000077_cash_orders_product_register`. Both are needed before running the updated application. The register migration uses PostgreSQL 15+ `NULLS NOT DISTINCT` constraints.
2. Deploy the updated backend and admin frontend. Restart both backend executables if the admin and MAX services run separately against the shared database.
3. Deploy the included MAX compatibility update when that mini-app is used. It can display an order without a sale place; cash orders are administered in the admin frontend. Copying one into a new MAX order uses the normal order flow and requires selection of a sale place.
4. Install the editable 1C module changes described in [Order customer contacts in 1C](order-customer-contacts-1c.md) before exporting buyer contacts. The binary `WebAPI.epf` has not been rebuilt.

The flag does not register a payment or create a fiscal receipt. Buyer contacts are written to the invoice comment and copied to new shipment comments. The existing invoice-before-shipment workflow remains in use. Shipment creation checks that the latest requested invoice synchronization succeeded and pins its version; the updated 1C module checks that version before creating a new shipment. Existing shipments are not rewritten after later contact corrections.

## Code generation

`schema/order.yaml` defines the persistent fields, sale-place nullability, database check, list projection, generated form metadata, and column labels. The changed generated Go/TypeScript files were regenerated from that definition. Continue using the project's normal `make codegen` workflow; do not patch generated output manually.

The order aggregate, conditional validation, custom admin form, order-line projection, SQL views, and 1C mapping are maintained separately. YAML changes alone cannot replace those parts. Existing migrations remain immutable; migrations 76 and 77 apply the changes to existing databases.

Rollback is guarded: assign sale places to cash orders and clear nullable register movements through the normal application workflow before restoring the former mandatory dimensions. The down migrations fail instead of deleting orders or assigning placeholder places.

## Verification

Go tests cover optional fields, default behavior, required customer/sale place, cash detail loading, MAX read-only details, and 1C snapshots. Admin verification covers form toggling, empty contacts, serialization, DTO parsing and copying. Both frontends pass type checking and production builds.

The disposable PostgreSQL regression scripts are `internal/tests/cash_order_contacts.sql` and `internal/tests/cash_order_product_register.sql`. Follow their instructions and use a new disposable database; they are not production maintenance scripts. Actual 1C execution requires verification in the target configuration.
