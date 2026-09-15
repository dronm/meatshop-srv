package models

import "time"

// OrderDocument is the writable Order aggregate accepted by create and update.
// Reference projections are deliberately excluded from the command model.
type OrderDocument struct {
	ID                  int                  `json:"id"`
	Version             int64                `json:"version"`
	ForDate             time.Time            `json:"for_date"`
	Number1C            *string              `json:"number_1c"`
	Ref1C               *Ref1c               `json:"ref_1c"`
	ShipmentRef1C       *Ref1c               `json:"shipment_ref_1c"`
	CustomerID          int                  `json:"customer_id"`
	CustomerSalePlaceID int                  `json:"customer_sale_place_id"`
	CustomerUserID      *int                 `json:"customer_user_id"`
	StatusID            *int                 `json:"status_id"`
	CommentCustomer     *string              `json:"comment_customer"`
	CommentAdmin        *string              `json:"comment_admin"`
	Items               []*OrderDocumentItem `json:"items"`
}

// OrderDocumentItem is one writable row embedded into OrderDocument.
type OrderDocumentItem struct {
	ID            int     `json:"id"`
	LineNum       int     `json:"line_num"`
	ProductID     int     `json:"product_id"`
	MeasureUnitID int     `json:"measure_unit_id"`
	QuantRequired float64 `json:"quant_required"`
	Quant         float64 `json:"quant"`
}

// OrderDetail is the complete Order aggregate returned by detail, create, and
// update operations. It contains both foreign key values and their reference
// projections so clients can initialize reference inputs without extra calls.
type OrderDetail struct {
	ID                  int                `json:"id"`
	Version             int64              `json:"version"`
	ForDate             time.Time          `json:"for_date"`
	Number1C            *string            `json:"number_1c"`
	Ref1C               *Ref1c             `json:"ref_1c"`
	ShipmentRef1C       *Ref1c             `json:"shipment_ref_1c"`
	CustomerID          int                `json:"customer_id"`
	Customer            *Ref               `json:"customer"`
	CustomerSalePlaceID int                `json:"customer_sale_place_id"`
	CustomerSalePlace   *Ref               `json:"customer_sale_place"`
	CustomerUserID      *int               `json:"customer_user_id"`
	CustomerUser        *Ref               `json:"customer_user"`
	StatusID            *int               `json:"status_id"`
	Status              *Ref               `json:"status"`
	CommentCustomer     *string            `json:"comment_customer"`
	CommentAdmin        *string            `json:"comment_admin"`
	Items               []*OrderDetailItem `json:"items"`
}

// OrderDetailItem is one Order item returned with its reference projections.
type OrderDetailItem struct {
	ID            int     `json:"id"`
	LineNum       int     `json:"line_num"`
	ProductID     int     `json:"product_id"`
	Product       *Ref    `json:"product"`
	MeasureUnitID int     `json:"measure_unit_id"`
	MeasureUnit   *Ref    `json:"measure_unit"`
	QuantRequired float64 `json:"quant_required"`
	Quant         float64 `json:"quant"`
}

type UpdateOrderDocumentRequest struct {
	ID       int
	Document *OrderDocument
}
