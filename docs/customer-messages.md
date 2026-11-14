# Customer messages from MAX

Migration `000074_customer_messages` adds a durable inbox for incoming MAX text
messages and independent read state for each admin. Apply it before restarting
the main `meatshop` backend. No new configuration or MAX webhook subscription is
required when the bot already receives `message_created` updates.

## Storage and ingestion

- `max_in_messages` remains the complete raw update log.
- `customer_messages` contains one normalized message per unique MAX message ID.
- `customer_message_reads` contains `(message_id, user_id, read_at)` acknowledgements.

An insert trigger on `max_in_messages` extracts non-blank text from incoming
`message_created` updates in direct conversations. Bot senders, group messages,
service updates and messages containing only attachments remain in the raw log.
Text accompanying attachments is retained. The full original text, including
line breaks, is preserved. Valid MAX timestamps are converted from milliseconds;
missing or invalid timestamps fall back to the raw update's receipt time.

Extraction and the raw insert share a transaction. This works with the existing
`cmd/max` executable as well as a rebuilt one. Repeated delivery keeps raw update
history but creates neither duplicate inbox entries nor duplicate notifications.
Existing eligible raw updates are imported when the migration runs, without
emitting individual creation events. Imported messages are initially unread.

The sender's external MAX ID is retained even before registration. Customer
lookup uses `max_users.max_user_id`; the customer ID and sender name are snapshots.
Triggers attach previously unassigned messages when the sender becomes linked to
a customer, through either mini-app registration or the admin API. Messages
already assigned to a customer retain their association if the MAX user is later
linked to a different customer. Advisory transaction locks coordinate ingestion
and registration, including a sender's first registration. Removing the raw
update or customer does not delete the message; the corresponding reference is
set to null. Removing a MAX user also preserves messages.

## API

All endpoints require an authenticated admin. HTTP and direct WebSocket calls
enforce the same permissions. The admin ID is obtained from the session and
cannot be supplied in the request.

| Method and path | Permission | Result |
| --- | --- | --- |
| `GET /api/customer-messages` | `customerMessage.list` | Standard collection response: `rows` and `agg.tot_count` |
| `GET /api/customer-messages/{id}` | `customerMessage.detail` | Full stored message, including raw update reference |
| `GET /api/customer-messages/unread-count` | `customerMessage.list` | `{"unread_count": 3}` |
| `POST /api/customer-messages/mark-read` | `customerMessage.markRead` | `{"rows_affected": 2}` |

The list uses standard `filter`, `sorter`, `from` and `count` collection parameters.
It defaults to 30 rows, ordered by `id DESC`; the maximum page size is 200. An ID
sort is appended to other sorters for stable pagination. Each row includes full
text, sender, customer reference, timestamps, `is_read` and `read_at`. Read fields
are joined for the current admin inside the handwritten list method, rather than
stored globally in the collection view.

For a customer tab containing unread messages, the filter JSON is:

```json
{
	"filter": [{
		"j": "and",
		"f": {
			"customer_id": {"o": "e", "v": 123},
			"is_read": {"o": "e", "v": false}
		}
	}],
	"count": 30
}
```

Omit `is_read` to list all messages for a customer. Other supported fields include
`max_user_id`, `sender_name`, `text`, `sent_at` and `created_at`. For example,
`text` with operator `ilk` and value `%delivery%` performs a substring search.
Unassigned messages can be found with `customer_id` operator `i` and value `null`.
The count endpoint optionally accepts `?customer_id=123` for a customer tab badge.

Mark specific messages as read using:

```json
{
	"message_ids": [12, 13]
}
```

There must be 1–500 positive IDs. Repeated IDs are deduplicated; repeated requests
are idempotent and preserve the original read timestamp. If any selected message
does not exist, the request returns 404 and changes no read state. Messages that
arrive after the UI loaded its selection remain unread. Reading list/detail or
count endpoints never changes read state. Another admin's read status is unaffected.

## Live events

Subscribe through the existing `Event.Subscribe` WebSocket service:

```json
{
	"f": "Event.Subscribe",
	"q": "customer-message-subscription",
	"p": {
		"events": ["CustomerMessage.Create", "CustomerMessage.Update", "CustomerMessage.Read"]
	}
}
```

| Event | Payload | Frontend action |
| --- | --- | --- |
| `CustomerMessage.Create` | `id`, nullable `customer_id`, `max_user_id` | Refresh count and fetch message for a toast |
| `CustomerMessage.Update` | `max_user_id`, `customer_id` | Refresh customer association/history |
| `CustomerMessage.Read` | `user_id` | Refresh read state if this is the current admin |

Events are emitted after commit and carry only compact identifiers, never full
message text or raw MAX JSON. Only admins with `customerMessage.list` may
subscribe. The read event is an invalidation shared with authorized admin
subscribers; the frontend should ignore it when `user_id` is another admin.

PostgreSQL notifications are not a durable queue. On login, reconnect and window
focus, reload the count and unread collection through HTTP. Subscribe first and
then fetch the current state; deduplicate live toasts by message ID. A toast does
not mark a message as read. Acknowledge only messages whose full text was actually
displayed in an active page. Existing messages should produce one summary toast.

## Code generation

`schema/customerMessage.yaml` defines the stored and list models, generated
service registration, list/detail routes, permissions and future frontend
collection scaffolding. `service.manualMethods` preserves the custom list and
detail implementations when code generation runs. The migration is hand-maintained
because it owns extraction, historical backfill, read tracking and notification
triggers. Read tracking has no general-purpose CRUD API.

The frontend inbox, toast and customer tab are a separate implementation step.
Migration 74 registers the application route with `menu_available = false` and
does not add a menu item, so this backend can be deployed before the inbox page.
Navigation will be enabled when the frontend is added.

## Verification

Run `go test ./...` for unit tests and `go vet ./...` for static checks.
To include PostgreSQL integration tests, provide a disposable PostgreSQL server
and a user with `CREATEDB`:

```sh
MEATSHOP_TEST_DATABASE_URL='postgres://test_user@127.0.0.1:5432/postgres?sslmode=disable' \
	go test ./...
```

Each integration test creates and drops its own database. Tests cover historical
import, duplicate delivery, text filtering, per-admin read state, read retries,
atomic rejection of missing IDs, concurrent sender registration, transactional
notifications, and the up/down/up migration sequence. WebSocket tests exercise
admin delivery and rejection of customer subscriptions and direct inbox calls.

Rolling migration 74 back removes the inbox and read records. The raw MAX update
log is retained, so applying it again rebuilds message history as unread.
