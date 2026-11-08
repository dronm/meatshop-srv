# Order status workflow

Apply migration `000067_order_status_workflow` and deploy the backend and admin
frontend together. From database version 66, `make migup` applies this migration
(the Makefile advances one migration per call).

| ID | Existing code | Display name | Sort order | Final |
| --- | --- | --- | --- | --- |
| 1 | `new` | Новый | 10 | No |
| 3 | `processing` | В обработке | 20 | No |
| 2 | `accepted` | Готовим | 30 | No |
| 4 | `approved` | На доставке | 40 | No |
| 5 | `completed` | Выполнен | 60 | Yes |
| 6 | `cancelled` | Отменен | 60 | Yes |

The migration changes display names and sort order. Existing IDs, codes, final
flags, and order status references are preserved. Existing orders are not moved
between statuses automatically; for example, a legacy MAX order in `new` remains
there until its status is changed. Orders already in `accepted` or `approved`
show the new display names.

## Creation and customer changes

- MAX submissions start in `processing` (В обработке), resolved by code on the
  server. Customers can correct an order while it is `processing` or legacy
  `new`. The list/detail `can_change` field and update validation use the same
  rule. Moving to `accepted` (Готовим) closes customer editing.
- New and copied admin order forms start in `new` (Новый). The admin creation
  service also defaults an omitted/null status to `new`, resolved by code.
  An explicitly selected status is preserved.
- After checking availability, the manager selects Готовим. When the order
  leaves for delivery, the manager selects На доставке. These changes use the
  existing order form or batch status command. No inventory calculation or 1C
  action automatically advances these statuses.

## Code dependencies checked

`MaxMiniAppService.CreateOrder` selects `processing` by code.
`defaultAdminOrderStatus` selects `new` by code.
`canCustomerChangeOrder` permits only `new` and `processing`.
The completed-history import uses `completed`, and demo SQL uses `approved`.
Keeping all existing codes avoids changes to those scripts and existing data.

1C order creation, correction synchronization, shipment creation, printing, and
batch status changes do not require `accepted` or `approved` semantics. Status
notifications read the display name from the database. The existing status
change service continues to accept any existing status ID; this change does
not impose a transition graph or enforce `is_final`.

## Admin labels and colors

The shared `OrderStatusReferenceInput.vue` selector uses the workflow order
above for both the order form and batch status dialog.

`src/collections/order.ts` and `src/collections/orderLineList.ts` both use
`src/utils/orderStatus.ts`. Their existing ID-based color classes and CSS are
preserved:

| Status | Color |
| --- | --- |
| Новый | Pale blue |
| В обработке | Pale amber |
| Готовим | Pale cyan |
| На доставке | Pale violet |
| Выполнен | Pale green |
| Отменен | Pale red |

Selection highlighting retains priority over status background colors.

## Validation

Regression tests in `internal/services/orderStatus_test.go` cover the customer
editing boundary, lookup of the admin default by code rather than numeric ID,
preservation of explicit admin selections, and propagation of missing status
errors. Run `make test` with the project's local Go dependencies available.
Run `npm run build` in the admin frontend for type checking and bundling.
