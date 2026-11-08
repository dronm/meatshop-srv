# Customer marking setting

Customers have an editable `use_marking` boolean. Migration `000069_customers_use_marking` adds it as `NOT NULL DEFAULT false`, so existing customers and newly created customers start with marking disabled unless selected explicitly. The customer edit form exposes the setting as `Маркировка`, and the customer collection includes the same field.

The source definition is `schema/customer.yaml`: it declares the stored field, its default, the customer-list projection, the checkbox and its list column. The generated customer Go model and frontend artifacts were regenerated from this schema. Custom customer editor code still supplies its existing controls and submit whitelist.

The order preparation collection keeps its existing JSON field and checkbox, `use_marking`, but the `order_lines_list` SQL view now obtains that value from `customers.use_marking`. Every visible line belonging to a customer shows that customer's setting, regardless of the item-specific 1C flag. A missing customer resolves to `false` through `COALESCE`.

`order_items.use_marking` is retained independently. Order item models and 1C result handling continue to use the existing item-specific flag. Changing the customer setting does not rewrite item flags, quantities, requested quantities, prices or order statuses.

Migration `000069` appends the new customer-list field while preserving all existing view columns, including `allow_holiday_orders`. It retains the preparation list's positive-quantity filter, status, comments, item identity and sorting. The down migration restores the previous item-based preparation projection and customer-list view before dropping the customer field.

The corresponding canonical SQL sources are `migrations/sql/customers_list.sql` and `migrations/sql/order_lines_list.sql`.
