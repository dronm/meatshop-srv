# MAX bot

The MAX bot executable lives in `cmd/max`.

It has two responsibilities:

1. receive MAX webhook updates and persist every valid `Update` into `public.max_in_messages`;
2. consume `public.max_out_messages` and deliver queued notifications through MAX Bot API.

On `bot_started`, the webhook transaction also upserts `public.max_users` and queues the hardcoded welcome message with an `open_app` button.

## Configuration

`config.json` uses the `max` section:

```json
{
  "max": {
    "bot_token": "...",
    "http_addr": "127.0.0.1:59001",
    "webhook_url": "https://example.ru/max/webhook",
    "webhook_secret": "...",
    "mini_app_url": "",
    "init_data_max_age": "1h",
    "sender_poll_interval": "2s",
    "sender_notify_reconnect_interval": "5s",
    "sender_lock_timeout": "1m",
    "sender_retry_base_delay": "5s",
    "sender_retry_max_delay": "5m",
    "sender_max_attempts": 5
  }
}
```

When `webhook_url` is non-empty, `cmd/max` configures the MAX webhook subscription on startup. Leaving `update_types` unspecified subscribes to all update types.

`mini_app_url` is optional. When omitted, the `open_app` button relies on the Mini App configured for the bot in MAX. When set, it is sent as the `web_app` field of the button.

## Mini-app customer registration

The customer lookup step uses a JSON POST request:

```text
POST /api/max/registration/customer
```

```json
{
  "inn": "7701234567",
  "app_username": "Иван Иванов"
}
```

Both values are trimmed and must be non-empty. The customer is selected by INN;
KPP remains part of the customer data but is not used as a registration input.
The selected customer and application username are kept in the session while the
user selects a sale place. `POST /api/max/registration` then saves the customer,
default sale place, and application username together.

Before registration, a newly discovered MAX user receives an initial
`app_username` from the MAX platform username. If MAX does not provide a
username, `Не задано` is used. Completing registration replaces this provisional
value, and subsequent MAX session or bot updates preserve the registered name.

## Admin notifications

The main API exposes:

```text
POST /api/max/notifications
```

with permission `maxNotification.send`.

Example request:

```json
{
  "max_user_id": 123456789,
  "text": "Ваш заказ принят в работу.",
  "metadata": {
    "source": "order",
    "order_id": 123
  }
}
```

The API only enqueues the notification. `cmd/max` performs delivery and updates queue status/retry information.

## Build/run

```bash
make build-max
make run-max
make prod-max
```
