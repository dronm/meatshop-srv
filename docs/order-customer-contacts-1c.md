# Order customer contacts in 1C

An order still belongs to the selected Meatshop customer and uses that customer's existing 1C counterparty reference. `customer_name` and `customer_phone` describe the actual buyer of this order; they do not create or modify counterparties. Both fields are optional. The `cash_payed` flag is included in the integration snapshot for consistency, but it does not create a cash receipt, register a payment, or change document posting.

## Command contract

Both `create_order` and each order inside `create_orders.params.orders` accept these additional fields:

```json
{
	"cash_payed": true,
	"customer_name": "Иван Иванов",
	"customer_phone": "+7 (912) 123-45-67"
}
```

Missing or null contacts are accepted. Whitespace-only contacts are treated as empty by the backend. Existing jobs without the new fields remain compatible. Contacts are stored in the queued order snapshot together with its version; later order changes use the existing versioned correction mechanism. Result application checks the counterparty, contact snapshot, cash flag, and item snapshot against the current order.

## 1C documents

The module writes the invoice's integration-owned `Комментарий` as:

```text
Заказ №101
Покупатель: Иван Иванов
Телефон: +7 (912) 123-45-67
```

Empty contact lines are omitted. The complete comment is assigned once on each new order version, so corrections replace old contacts and never append duplicate lines. An equal-version retry checks the saved comment for new-format requests and returns the existing result without rewriting it. Send a newer local order version to correct contact information; do not edit this integration-owned comment independently in 1C.

`create_shipments` requires the invoice to exist first. The backend locks all selected orders and rejects shipment creation while any invoice synchronization is queued/processing, or when the latest requested synchronization has no successfully consumed response for the current order snapshot. HTTP/business failures, obsolete/skipped responses, missing calculations, and unconsumed results do not make an order ready for shipment. Previously linked/imported invoices with no queue history remain usable.

For synchronized invoices, the backend adds the accepted versions to the shipment command:

```json
{
	"order_ids": [101],
	"order_versions": [{"order_id": 101, "order_version": 2}]
}
```

A newly created shipment copies the saved invoice comment, including the real buyer contacts. The 1C module checks each pinned invoice version while holding its shared order lock through shipment creation. If a correction changes that invoice before a delayed shipment job runs, the job fails instead of silently creating a shipment from a different version; synchronize and request the shipment again. New shipment job identities include the version snapshots, so a fresh request does not reuse an older-version active job. An accepted shipment request describes the invoice snapshot at enqueue time. Legacy `order_ids`-only jobs remain compatible, and imported invoices without queue history have no pinned version.

Existing shipments keep their current no-rewrite retry behavior. Editing the order after a shipment already exists updates the invoice, but does not rebuild, change, unpost, or repost that shipment. If its contact information needs changing, update that existing document through the normal 1C process. Printing behavior is unchanged: contact information is stored in the documents' comments; its visibility in a printed form depends on that form's layout.

## Install the 1C module

The editable source is `documents/1c/ObjectModule(1).bsl`. The packaged binary `WebAPI.epf` has not been rebuilt.

1. Make a backup of the external processing currently used by the connector.
2. Open that processing in 1C Configurator and open its object module. Compare it with the supplied BSL source and transfer the contact changes. Preserve the organization, warehouse, pricing, VAT, marking, shipment, and printing adaptations used by your configuration.
3. The changed or added routines are `ПолучитьНеобязательнуюСтроку`, `СформироватьКомментарийСчета`, `ПроверитьКомментарийПовторногоЗаказа`, `ЗаполнитьШапкуСчета`, `ПодготовитьДанныеСозданияЗаказа`, `СоздатьСчетПоПодготовленномуЗаказу`, `ЗаполнитьШапкуОтгрузки`, `ПолучитьВерсииСчетовДляОтгрузки`, `ПроверитьВерсиюСчетаПередОтгрузкой`, and `ВыполнитьСозданиеОтгрузок`.
4. Check the module syntax, save the updated processing as `WebAPI.epf`, and update the processing actually registered/loaded by your connector. Restart or reload it using your existing deployment procedure.
5. Stop or pause the backend integration worker during the update. Install/reload the updated 1C module first, then apply the backend database migration, deploy the backend/frontend, and resume the worker. An older 1C module can ignore contacts and pinned invoice versions, so deploying only the backend does not provide the shipment-version guarantee.

No additional 1C document requisites are required for this implementation; it uses the existing `Комментарий` fields. Ensure their configured lengths can hold the complete contact text and that configuration write handlers preserve it.

## Verification

Backend tests cover single/batch JSON compatibility, queued correction snapshots, empty contacts, batch correlation changes, rejection of stale contact snapshots, and shipment readiness across queued/processing/failed/skipped/unconsumed synchronization results. Disposable PostgreSQL tests exercise the enqueue gate, legacy references without jobs, pinned versions, and waiting for a concurrent result-consumer transaction to commit. A structural BSL regression checks that a delayed shipment must pass its pinned-version check inside the shared-lock transaction before creation; this does not execute the 1C runtime.

A 1C runtime is not available in the development container. Check these cases in a test 1C database before production deployment:

1. Create an invoice with both contacts; confirm the selected counterparty remains unchanged and both lines appear in `Комментарий`.
2. Retry the same version; confirm the same invoice is returned and contact lines are not duplicated.
3. Change or clear contacts in Meatshop; confirm a newer version replaces or removes those lines.
4. Create a shipment after synchronization; confirm its comment contains the saved buyer contacts.
5. Retry an existing shipment after another invoice correction; confirm the existing shipment is returned unchanged.
6. Send an ordinary order or an old queued payload without contacts; confirm it continues to work.
7. Edit a contact and request a shipment while invoice synchronization is queued/retrying or has failed; confirm the backend blocks it. After successful result consumption, confirm a fresh shipment request is accepted.
8. Delay a shipment job after enqueue, synchronize a newer invoice version, and then execute the old shipment job; confirm no new shipment is created from the wrong version and request a fresh shipment.
