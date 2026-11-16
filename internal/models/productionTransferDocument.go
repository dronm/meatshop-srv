package models

import "time"

// ProductionTransferDocument combines its header with live order projections.
type ProductionTransferDocument struct {
	ID                 int                   `json:"id"`
	Version            int64                 `json:"version"`
	DocumentDate       time.Time             `json:"document_date"`
	ForDate            time.Time             `json:"for_date"`
	Comment            *string               `json:"comment"`
	Orders             []*ProductionOrderRow `json:"orders"`
	NotificationStatus string                `json:"notification_status"`
	NotificationError  *string               `json:"notification_error"`
}

type ProductionOrderRow struct {
	ID                int                   `json:"id"`
	TransferID        int                   `json:"transfer_id"`
	OrderID           int                   `json:"order_id"`
	LineNum           int                   `json:"line_num"`
	Version           int64                 `json:"version"`
	Order             *Ref                  `json:"order"`
	ForDate           time.Time             `json:"for_date"`
	Number1C          *string               `json:"number_1c"`
	Customer          *Ref                  `json:"customer"`
	CustomerSalePlace *Ref                  `json:"customer_sale_place"`
	Products          string                `json:"products"`
	StatusCode        string                `json:"status_code"`
	CommentCustomer   *string               `json:"comment_customer"`
	CommentAdmin      *string               `json:"comment_admin"`
	Items             []ProductionOrderItem `json:"-"`
}

type ProductionOrderItem struct {
	ProductID       int
	ProductName     string
	MeasureUnitID   int
	MeasureUnitName string
	Quant           string
	UseMarking      bool
}

type ProductionTransferInput struct {
	Version       int64         `json:"version"`
	DocumentDate  string        `json:"document_date"`
	ForDate       string        `json:"for_date"`
	Comment       *string       `json:"comment"`
	OrderIDs      []int         `json:"order_ids"`
	OrderVersions map[int]int64 `json:"order_versions,omitempty"`
	RequestID     string        `json:"request_id,omitempty"`
}

type ProductionTransferUpdateRequest struct {
	ID       int
	Document *ProductionTransferInput
}

type ProductionTransferPrepareRequest struct {
	OrderIDs []int `json:"order_ids"`
}

type ProductionTransferSkippedOrder struct {
	OrderID int    `json:"order_id"`
	Reason  string `json:"reason"`
}

type ProductionTransferPreview struct {
	ForDate *time.Time                       `json:"for_date"`
	Orders  []*ProductionOrderRow            `json:"orders"`
	Skipped []ProductionTransferSkippedOrder `json:"skipped"`
}

type ProductionOrderOptionsInput struct {
	Search  string
	OrderID int
	From    int
	Count   int
}

type ProductionTransferReference struct {
	ID int `json:"id"`
}
