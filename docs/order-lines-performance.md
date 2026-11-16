# Order-lines query performance

Apply migration `000089_order_lines_query_performance` to existing databases.
The canonical definitions in `migrations/sql` include the same changes for
environments that install projection SQL separately.

The order-lines projection previously calculated its optional production
document with a correlated scalar subquery for every projected item. The
projection also called four JSON reference builders declared `VOLATILE`, which
prevented PostgreSQL from discarding their work in the collection's count query.

The forward migration:

- Uses the existing unique production-order index through an optional join.
- Uses an inner join for the mandatory order parent. `order_items.order_id` is
	`NOT NULL` and references `orders`, so this preserves the returned rows while
	allowing an index-driven query for the newest orders.
- Declares `customers_ref`, `customer_sale_places_ref`, `products_ref` and
	`order_statuses_ref` as `STABLE`. Each function only builds JSON from its input
	row; its result does not change during one query.

The projection's existing order is preserved: order ID descending, then line
number and item ID. Item identity, status/quantity fields, reference filtering
and production-document references are unchanged.

An isolated PostgreSQL 18.6 benchmark used synthetic data: 10,000 orders,
50,000 active items and 2,000 production links. A representative warm execution
measured the full-list count and first 30-item page as follows:

| Query | Previous projection | Updated projection |
| --- | ---: | ---: |
| Count | 505 ms | 57 ms |
| First page | 577 ms | 15 ms |
| Combined database work | 1,082 ms | 72 ms |

These measurements describe the synthetic fixture, not production latency.
The regression test verifies row preservation, paging identity, optional
production references and the absence of a correlated subplan without relying
on machine-specific timing thresholds.

## Frontend deployment

Update the admin frontend and the local `@katren/vue-collection-lib` source,
then rebuild the admin frontend. The grid changes reuse unchanged cell values
when the cursor moves, reuse column pass-through objects, avoid redundant
selection updates and batch cursor-only session-storage writes. Marking,
paging and filter persistence remain immediate; pending cursor state is flushed
when leaving the page or unmounting the grid.

Order-lines requests have a dedicated 30-second timeout. A recoverable Refresh
failure remains visible in the collection's error panel without escaping as an
unhandled Vue command-event rejection. Other API requests keep their existing
timeout settings.
