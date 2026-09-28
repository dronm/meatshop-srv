package models

import wmodels "github.com/dronm/webapp/models"

const orderLineListRelation = "public.order_lines_list"

// OrderLineList is one order item in the order_lines_list collection view.
// ID is the parent order ID and may repeat for several lines; ItemID is unique.
type OrderLineList struct {
	ID                int      `json:"id"`
	ShipmentRef1C     *Ref1c   `json:"shipment_ref_1c"`
	Customer          *Ref     `json:"customer"`
	CustomerSalePlace *Ref     `json:"customer_sale_place"`
	Product           *Ref     `json:"product"`
	Quant             float64  `json:"quant"`
	Price             *float64 `json:"price"`
	Amount            *float64 `json:"amount"`
	VatAmount         *float64 `json:"vat_amount"`
	UseMarking        bool     `json:"use_marking"`
	ItemID            int      `json:"item_id" primaryKey:"true"`
	LineNum           int      `json:"line_num"`
	Ref1C             *Ref1c   `json:"ref_1c"`
}

func (m OrderLineList) Relation() string {
	return orderLineListRelation
}

func (m OrderLineList) CollectionAgg() any {
	return &wmodels.TotCount{TotCount: 0}
}
