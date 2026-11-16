# Production transfers

The admin application supports editable **Передача в производство** documents.
An order or any of its lines can be selected in either order collection and sent
with **Передать в производство**. Selecting several items of one order includes
that whole order once.

## Deployment

1. Apply all new migrations **000082–000088**, in order, to an existing database
   at version 81. `make migup` advances only one migration per invocation.
2. Deploy the backend, MAX bot executable and admin frontend together.
3. Set **Чат MAX для производства** (`production_max_chat_id`) on the constants
   page. Paste the complete signed decimal chat ID. It is stored as text to
   preserve all 64-bit digits.
4. In MAX, allow the bot to be added to groups, add it to the production group or
   channel, and grant the required administrator reading and writing rights.
   The webhook receives `bot_added`; the MAX process logs its `chat_id` and the
   incoming-events collection also records it. Never use the bot's own user ID
   instead of the destination chat ID.

The bot uses `https://platform-api2.max.ru` with its token in the Authorization
header. This feature does not send a test message automatically.

## Document editing

The document contains a document date, an order date and an optional comment.
Its standard order collection supports adding, replacing and removing order
references. The order reference displays the 1C number (or internal number when
missing) and order date, with autocomplete and list selection.

Changes to this collection are staged until the parent document is saved.
Removing a row does not delete its source order. All included orders must have
one order date. New additions that are already in production or have progressed
past preparation are skipped with an explanation. Orders already in the current
document remain available regardless of their current status.

Only references are stored. Product quantities, customer and sale-place names,
comments, collection totals and printouts are read from the actual orders.
Consequently, later changes to an order are reflected on reopening or printing
the production document. Zero-quantity items are omitted. Totals are grouped by
product and item measurement unit, with kilograms first, boxes second and other
units afterwards.

## Reversible status assignments

`order_statuses` remains the catalogue. `order_status_assignments` records each
order's assignments, in ID order, including the source document and whether an
assignment is still active. Existing orders receive their current value as a
baseline; nullable statuses are preserved.

Adding an order assigns **Готовим** with recorder
`{"document_type":"production_transfer","document_id":123}`. Removing it or
deleting the document withdraws only that document's assignment. The latest
remaining active assignment determines `orders.status_id`. Later manual changes
are preserved, even when the manually selected status was also **Готовим**.
Re-adding an order creates a new assignment. Saving unchanged membership does
not repost statuses.

Automatic database tracking covers order creation and actual status changes,
including imported orders. The explicit batch status command records manual
intent even for the same value. Ordinary order edits with unchanged status do
not replace document ownership.

The **История статусов** tab on a saved order shows timestamps, statuses,
source-document links, and current, previous or withdrawn assignments. Withdrawn
assignments are retained; the new business tables are also registered in audit.

## MAX delivery and printing

Saving a changed document queues the detailed customer/order message followed by
unit-grouped totals. Messages are split at Unicode character boundaries within
MAX's 4,000-character limit. Queue ordering prevents later parts overtaking a
part awaiting retry. The existing sender rate limit remains in effect.

The configured destination and message text are fixed when queued. Editing the
document supersedes unsent older versions and queues an updated version.
Previously delivered messages are retained in MAX. Network timeouts can make
remote delivery ambiguous, so exactly-once remote delivery is not guaranteed.
The retry action retries failed parts of the current version without resending
successful parts. Deleting a document cancels unsent messages.

The document has a separate HTML print form, suitable for A4 landscape or the
browser's PDF printing. Each order has a customer/sale-place heading and its
current header comments, followed by item rows. **Маркировка** is taken from each
`order_items.use_marking`, independently of the customer-level preparation flag.
Printing does not change statuses or queue messages.

## Generation

The three table definitions, read projections' DTOs, shared constant and generated
registries are represented in `schema/*.yaml`. Their initial table migrations
were generated with codegen; supplemental migrations provide views, audit and
status tracking. Custom files implement document transactions, printing, live
order selection and MAX formatting. Do not hand-edit generated `.gen` files.
