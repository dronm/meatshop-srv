# Application constants

Meatshop uses `github.com/dronm/webapp/constants` for configuration that an
administrator can change while the backend is running. The first constant is:

| Code | Display name | Go value | Initial value |
| --- | --- | --- | --- |
| `cash_customer` | Покупатель за наличный расчёт | `*models.Ref` referencing `public.customers` | `nil` / JSON `null` |

A configured value uses the project's existing reference shape:

```json
{
	"keys": {"id": 42},
	"descr": "Покупатель за наличный расчёт"
}
```

`keys.id` must be positive and identify an existing customer in the primary
database. JSON `null` clears the selection. `descr` is a display snapshot; the
customer ID is the identity. No customer is created or selected automatically.
This change supplies constant storage and access; it does not apply the selected
customer to orders yet.

## Installation

The local `github.com/dronm/webapp` checkout must include its `constants` package.
The project retains its existing `go.mod` local replacements.

Apply migrations before restarting either `meatshop` or `max`:

```sh
make migup
```

- `000078` installs the common constants table and notification/version triggers.
- `000079` seeds `cash_customer` without overwriting an existing selection.
- `000080` grants the constants API permissions to the administrator role.
- `000081` enforces the customer reference with a generated key column and a
	foreign key, including concurrent changes. Clear or change the constant before
	deleting its selected customer.

Both processes initialize one manager for their primary database and close it
before closing their database provider. Changes synchronize through PostgreSQL
LISTEN/NOTIFY, with periodic reconciliation. The configured primary DSN must
support a persistent LISTEN connection (direct PostgreSQL or session pooling).
The seeded row is required; its value can remain null while unconfigured.

Rolling back `000079` deletes this selection; rolling back `000078` deletes all
stored constants. Roll back these migrations only after stopping application
versions that require them.

## Administrator API

| Method | URL | Service method | Permission |
| --- | --- | --- | --- |
| GET | `/api/constants` | `Constant.List` | `constant.list` |
| GET | `/api/constants/{id}` | `Constant.Detail` | `constant.detail` |
| POST | `/api/constants/get` | `Constant.Get` | `constant.get` |
| POST | `/api/constants/set` | `Constant.Set` | `constant.set` |

Every service method also requires a non-banned administrator session, including
direct WebSocket calls. There is no create/delete API; migrations define the
catalogue. The list and detail methods return metadata. Use the numeric ID from
the list to read the current value:

```json
{"ids": [17]}
```

Use the returned version for optimistic concurrency when saving:

```json
{
	"items": [
		{
			"id": 17,
			"value": {"keys": {"id": 42}, "descr": "Покупатель"},
			"expected_version": 1
		}
	]
}
```

The example IDs are illustrative; numeric constant IDs can differ by database.
To clear the selection, supply `"value": null`. Writes in a batch are atomic.
An outdated `expected_version` returns a conflict so the caller can reload.

## Typed backend access and generation

`internal/appconstants.Open` returns a `Project` containing the process-wide
manager and `CashCustomer` accessor:

```go
customer, err := projectConstants.CashCustomer.GetValue(ctx)
if err != nil {
	return err
}
// customer == nil means that no cash customer has been configured.
```

The accessor also exposes `SetValue(ctx, *models.Ref)`. It and the generic API
share the same validation and storage. Backend callers are trusted; request
handlers must enforce authorization before making changes.

`schema/cashCustomer.yaml` is the source definition for constants codegen.
The registry, typed accessor, administration service/routes and shared frontend
editor are managed generated files. Startup calls in the existing `main.go`
files, the application WebSocket adapter, custom order form, and ordinary tests
remain application-owned.

Codegen records the generated content hashes in `.codegen/constants.json` under
both the backend and frontend roots. Commit and copy those manifests with the
generated files. A managed file whose bytes differ from its recorded hash is
protected from overwriting, even with `-adopt-constants`.

The repaired archives contain genuine generated constants files and their
matching ownership records. Apply both archives, including `.codegen`, then run
`make codegen` as usual. There is no need to adopt them again. Existing database
migrations are unchanged; this repair requires no database migration.

If a managed file is changed later, review the difference and preserve it in a
backup before moving the conflicting file out of the managed path and
regenerating it. Do not delete the ownership record or replace its hashes to
silence the check. Port intentional custom behavior to application-owned code
or update the generator's templates before regeneration.

The frontend's `npm run format` excludes `.gen.ts`, `.gen.vue` and `.gen.json`
outputs so formatting cannot invalidate their ownership hashes. Other formatter
or editor integrations must likewise leave generated outputs unchanged.
