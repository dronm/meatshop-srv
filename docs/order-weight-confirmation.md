# Confirming actual order weights

`products.requires_weight_confirmation` identifies products whose actual quantity must be checked before preparation is complete. It defaults to `false`. Enable it manually for the actual `Тушка` product; there is no product-name matching or automatic group rule.

`order_items.weight_confirmed` stores the manager's weighing decision separately from requested and actual quantities. Migration `000070_order_item_weight_confirmation` marks existing order items as confirmed so that historical orders do not become pending. The database default for newly inserted items is `false`; the application marks ordinary products as confirmed automatically.

The order detail, MAX order detail and preparation collection expose read-only `requires_weight_confirmation` and `weight_confirmed` booleans. The requirement comes from the product catalogue. Admin item commands accept only an optional `weight_confirmed` marker; they cannot supply or override the product requirement. Existing admin clients can omit the marker.

## Admin changes

New or copied required items start pending, including copies whose actual quantity already differs from the requested quantity. An explicit `weight_confirmed: true` confirms the manager's decision, including the `Без изменений` case where both quantities are equal.

For an existing required item:

- Changing its product or measurement unit starts a new pending decision unless explicitly confirmed.
- Changing its actual quantity confirms the weight when its product and unit stay the same.
- Changing only its requested quantity resets confirmation unless explicitly confirmed. Changing both requested and actual quantities confirms the actual edit.
- Keeping all quantities and identity unchanged preserves the existing marker when the field is omitted. An explicitly supplied marker is applied.

Ordinary products are always treated as confirmed. Reordering rows or editing order dates, comments or statuses does not confirm a pending weight. Confirmation-only saves do not generate a quantity-difference notification. Existing product-register, version and 1C correction handling still applies to saved orders.

The preparation collection can highlight a pending item using `requires_weight_confirmation && !weight_confirmed`; equality or inequality of the two quantities is not sufficient to determine confirmation.

## MAX changes

MAX creates required items as pending and ordinary items as confirmed. Customer input cannot contain either the confirmation marker or product requirement metadata.

Updating a MAX order preserves item identities instead of deleting and rebuilding all rows. Unchanged product, current measurement unit and requested quantity retain the actual quantity and confirmation marker, including an actual quantity of zero. This covers comment/date-only changes and row reordering. Changed requested quantities, newly selected products/units and new rows start with actual quantity equal to requested quantity and the appropriate new confirmation state.

Matching first reserves unique exact product/unit/requested-quantity matches, then reuses a remaining same-product/unit item only when it is unique. Existing admin-created duplicate rows remain separate. If several rows are equally eligible, the update returns a conflict asking the customer to contact the manager before writing any changes. MAX's existing rejection of duplicate product IDs in a request remains in place.

Removing a line from the preparation collection still sets its actual quantity to zero and preserves its confirmation marker; the positive-quantity filter hides it. The complete order editor retains its existing physical row removal behavior. Status changes, price invalidation and 1C result application do not set `weight_confirmed`.

## Admin interface and setup

Apply migration `000070`, rebuild both applications, then enable `Требует проверки веса` in the product editor for the actual `Тушка` catalogue record or records. The setting is stored on the product rather than inferred from its name.

The preparation collection and order item editor show only the pending quantity text in red, with the tooltip `Требуется проверить вес`. Existing row status backgrounds, line selection and order group boundaries are retained. Zero quantities are not highlighted.

Edit the order item row and enter the actual weighed quantity, or select `Без изменений` when its quantity matches the request. Commit the item row and save the order document to persist confirmation. Cancelled edits do not confirm a weight. Product details that cannot be loaded and unfinished or failed item saves block submission of stale item values.
