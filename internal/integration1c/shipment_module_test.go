package integration1c

import (
	"os"
	"strings"
	"testing"
)

// This is a structural regression check for the shipped BSL source, not a 1C
// runtime test. The version check must stay inside the same shared-order-lock
// transaction as new shipment creation, including delayed queue executions.
func TestShipmentModuleChecksPinnedInvoiceVersionUnderLock(t *testing.T) {
	module, err := os.ReadFile("../../documents/1c/ObjectModule(1).bsl")
	if err != nil {
		t.Fatal(err)
	}
	source := string(module)
	start := strings.Index(source, "Процедура ВыполнитьСозданиеОтгрузок(")
	if start < 0 {
		t.Fatal("shipment module entry point missing")
	}
	shipment := source[start:]
	shipment = shipment[:strings.Index(shipment, "КонецПроцедуры")]
	previous := -1
	for _, step := range []string{
		"ВерсииСчетов = ПолучитьВерсииСчетовДляОтгрузки(",
		"НачатьТранзакцию(",
		"ЗаблокироватьЗаказПоOrderID(",
		"Если СсылкаНаОтгрузку = Неопределено Тогда",
		"ПроверитьВерсиюСчетаПередОтгрузкой(",
		"Отгрузка = СоздатьОтгрузкуНаОснованииСчета(",
		"ЗаписатьОтгрузку(",
		"ЗафиксироватьТранзакцию(",
	} {
		index := strings.Index(shipment, step)
		if index < 0 || index <= previous {
			t.Fatalf("shipment lost version/lock ordering at %s", step)
		}
		previous = index
	}
	guardStart := strings.Index(source, "Процедура ПроверитьВерсиюСчетаПередОтгрузкой(")
	if guardStart < 0 {
		t.Fatal("pinned invoice version check missing")
	}
	guard := source[guardStart:]
	guard = guard[:strings.Index(guard, "КонецПроцедуры")]
	if !strings.Contains(guard, "Счет.order_version <> ОжидаемаяВерсия Тогда") || !strings.Contains(guard, "ВызватьИсключение(") {
		t.Fatal("a delayed shipment could accept a different invoice version")
	}
}
