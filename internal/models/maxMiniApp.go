package models

import "time"

type MaxSessionRequest struct {
	InitData string `json:"init_data"`
}

type MaxSessionUser struct {
	ID                  int     `json:"id"`
	MaxUserID           int64   `json:"max_user_id"`
	Username            *string `json:"username"`
	AppUsername         string  `json:"app_username"`
	CustomerID          *int    `json:"customer_id"`
	CustomerSalePlaceID *int    `json:"customer_sale_place_id"`
}

type MaxSessionResponse struct {
	Registered bool            `json:"registered"`
	User       *MaxSessionUser `json:"user"`
	Customer   *Ref            `json:"customer,omitempty"`
	SalePlace  *Ref            `json:"sale_place,omitempty"`
}

type MaxCustomerLookupRequest struct {
	INN         string `json:"inn"`
	AppUsername string `json:"app_username"`
}

type MaxCustomerLookupResult struct {
	ID   int     `json:"id"`
	Name string  `json:"name"`
	INN  string  `json:"inn"`
	KPP  *string `json:"kpp"`
}

type MaxSalePlace struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Address *string `json:"address"`
}

type MaxRegistrationRequest struct {
	CustomerID          int `json:"customer_id"`
	CustomerSalePlaceID int `json:"customer_sale_place_id"`
}

type MaxCatalogueScope string

const (
	MaxCatalogueScopeAll     MaxCatalogueScope = "all"
	MaxCatalogueScopeHistory MaxCatalogueScope = "history"
)

type MaxCatalogueItem struct {
	ID            int                 `json:"id"`
	ParentID      *int                `json:"parent_id"`
	Name          string              `json:"name"`
	Description   *string             `json:"description"`
	MeasureUnitID *int                `json:"measure_unit_id"`
	MeasureUnit   *Ref                `json:"measure_unit,omitempty"`
	IsGroup       bool                `json:"is_group"`
	Children      []*MaxCatalogueItem `json:"children,omitempty"`
}

type MaxOrderItemInput struct {
	ProductID     int     `json:"product_id"`
	QuantRequired float64 `json:"quant_required"`
}

type MaxOrderSubmitRequest struct {
	CustomerSalePlaceID int                 `json:"customer_sale_place_id"`
	ForDate             string              `json:"for_date"`
	CommentCustomer     *string             `json:"comment_customer"`
	Items               []MaxOrderItemInput `json:"items"`
}

type MaxOrderDateLimits struct {
	Available           bool     `json:"available"`
	MinimumForDate      string   `json:"minimum_for_date"`
	MaximumForDate      string   `json:"maximum_for_date"`
	UnavailableForDates []string `json:"unavailable_for_dates"`
	CutoffTime          string   `json:"cutoff_time"`
	BusinessTimezone    string   `json:"business_timezone"`
	AllowHolidayOrders  bool     `json:"allow_holiday_orders"`
}

type MaxOrderUpdateRequest struct {
	ID                  int                 `json:"-"`
	Version             int64               `json:"version"`
	CustomerSalePlaceID int                 `json:"customer_sale_place_id"`
	ForDate             string              `json:"for_date"`
	CommentCustomer     *string             `json:"comment_customer"`
	Items               []MaxOrderItemInput `json:"items"`
}

type MaxOrderListItem struct {
	ID                 int       `json:"id"`
	Version            int64     `json:"version"`
	ForDate            time.Time `json:"for_date"`
	Number1C           *string   `json:"number_1c"`
	StatusCode         string    `json:"status_code"`
	StatusName         string    `json:"status_name"`
	CustomerUser       *Ref      `json:"customer_user,omitempty"`
	CustomerSalePlace  *Ref      `json:"customer_sale_place,omitempty"`
	ItemsCount         int       `json:"items_count"`
	CanChange          bool      `json:"can_change"`
	HasQuantDifference bool      `json:"has_quant_difference"`
}

type MaxOrderItem struct {
	ID            int     `json:"id"`
	ProductID     int     `json:"product_id"`
	Product       Ref     `json:"product"`
	MeasureUnitID int     `json:"measure_unit_id"`
	MeasureUnit   *Ref    `json:"measure_unit,omitempty"`
	QuantRequired float64 `json:"quant_required"`
	Quant         float64 `json:"quant"`
	Price         *string `json:"price"`
	Amount        *string `json:"amount"`
	VatPercent    *string `json:"vat_percent"`
	VatAmount     *string `json:"vat_amount"`
	UseMarking    bool    `json:"use_marking"`
}

type MaxOrderDetail struct {
	ID                int             `json:"id"`
	Version           int64           `json:"version"`
	ForDate           time.Time       `json:"for_date"`
	Number1C          *string         `json:"number_1c"`
	Ref1C             *Ref1c          `json:"ref_1c"`
	StatusCode        string          `json:"status_code"`
	StatusName        string          `json:"status_name"`
	Customer          Ref             `json:"customer"`
	CustomerSalePlace Ref             `json:"customer_sale_place"`
	CustomerUser      *Ref            `json:"customer_user,omitempty"`
	CommentCustomer   *string         `json:"comment_customer"`
	CanChange         bool            `json:"can_change"`
	Items             []*MaxOrderItem `json:"items"`
}
