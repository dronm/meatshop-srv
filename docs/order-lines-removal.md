# Removing selected order lines from preparation

The admin order-lines collection removes selected item rows from preparation by setting their confirmed quantity (`order_items.quant`) to zero. It keeps the item row, its ID and line number, the original requested quantity (`quant_required`), and the parent order. No previous confirmed quantity is stored.

`POST /api/order/remove-lines` requires the existing `order.update` permission and accepts item IDs, including items belonging to different orders:

```json
{
	"item_ids": [101, 102, 201]
}
```

The response uses the standard `RowsAffectedResponse`; `rows_affected` counts item rows whose positive confirmed quantity changed to zero. Duplicate IDs are processed once. Empty selections, nonpositive IDs and selections larger than 1,000 IDs are rejected. Malformed JSON, unknown fields, extra JSON values and request bodies larger than 64 KiB are rejected.

The complete selection is processed in one PostgreSQL transaction. The service discovers parent orders, takes product-register advisory locks in order-ID order, and locks each order and its item snapshots. It validates every selected item again before the first update. A missing item, a concurrent editor removal/replacement, or a register, synchronization or notification error aborts the entire batch.

For each order containing changed rows, the service increments its version once, rebuilds its product-register actions with the retained requested quantities and zero confirmed quantities, and invokes the existing 1C correction synchronization. Orders already sent to 1C are synchronized again; unsent orders stay local. Existing correction handling clears stale prices, amounts and VAT values until the matching 1C result arrives. Zero-quantity rows remain in 1C requests.

The original MAX customer receives the existing `order.changed` notification with quantity differences such as `5 → 0`. One notification event is generated per changed order. The transaction emits one `Order.Update` PostgreSQL notification, which refreshes both order collections after commit. Already-zero items are no-ops: they do not increment versions or trigger register, synchronization or notification work.

Migration `000068_order_lines_list_active` filters the separate `order_lines_list` view to `quant > 0`. Zero rows disappear from the preparation collection and its totals. The order header collection and the complete order editor still retain those orders and items. Editing the quantity above zero makes the row visible again. The order editor's existing physical item removal behavior is unchanged. The down migration restores the previous unfiltered view.

## Admin collection commands

The preparation collection marks individual rows by `item_id`. Its context-menu command `Удалить позицию` sends the selected item IDs to the removal endpoint, including selections spanning several orders. It sets confirmed quantities to zero; the order editor continues to physically remove a row when its existing delete control is used.

The shared command factory in `src/collections/orderLineCommands.ts` preserves parent-order grouping for commands such as status changes and 1C operations. Selecting several lines from one order still targets that parent order once for those commands.

On the first load after this update, the collection clears previously saved order-based selection state so that an old order ID cannot select an unrelated item ID. Saved filters and sorting are preserved.
