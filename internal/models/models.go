package models

import "time"

// Order represents an internal store order
type Order struct {
	ID                  int64      `json:"id"`
	InvoiceNumber       string     `json:"invoice_number"`
	ClaimCode           string     `json:"claim_code"`
	ProductSlug         string     `json:"product_slug"`
	ProductName         string     `json:"product_name"`
	ProductType         string     `json:"product_type"`
	Quantity            int        `json:"quantity"`
	BasePriceUSD        float64    `json:"base_price_usd"`
	SellPriceIDR        int64      `json:"sell_price_idr"`
	OriginalAmount      int64      `json:"original_amount"`
	UniqueCode          int        `json:"unique_code"`
	TotalAmount         int64      `json:"total_amount"`
	GatewayOrderID      string     `json:"gateway_order_id"`
	QRURL               string     `json:"qr_url"`
	PaymentStatus       string     `json:"payment_status"`     // PENDING, PAID, EXPIRED, CANCELLED
	FulfillmentStatus   string     `json:"fulfillment_status"` // PENDING, SUCCESS, FAILED
	FulfillmentError    string     `json:"fulfillment_error"`
	DigitalCoreOrderID  string     `json:"digitalcore_order_id"`
	DigitalCoreViewURL  string     `json:"digitalcore_view_url"`
	DeliveredItems      string     `json:"delivered_items"` // JSON encoded array
	WarrantyInfo        string     `json:"warranty_info"`
	CustomerContact     string     `json:"customer_contact"`
	CustomerEmail       string     `json:"customer_email"`
	CostAtCheckoutUSD   float64    `json:"cost_at_checkout_usd"`
	CostAtFulfillmentUSD float64   `json:"cost_at_fulfillment_usd"`
	PaidAt              *time.Time `json:"paid_at"`
	ExpiresAt           *time.Time `json:"expires_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ProductTier represents wholesale pricing from DigitalCore
type ProductTier struct {
	MinQty   int     `json:"minQty"`
	MaxQty   *int    `json:"maxQty"`
	PriceUSD float64 `json:"priceUSD"`
	PriceIDR int64   `json:"priceIDR"`
}

// Product represents a catalog product presented to the frontend
type Product struct {
	RawID             interface{}   `json:"rawId,omitempty"`
	Slug              string        `json:"slug"`
	Name              string        `json:"name"`
	Type              string        `json:"type"`
	Logo              string        `json:"logo"`
	Region            string        `json:"region"`
	Stock             int           `json:"stock"`
	StockCapped       bool          `json:"stockCapped"`
	Warranty          string        `json:"warranty"`
	Description       string        `json:"description"`
	Terms             string        `json:"terms"`
	PriceUSD          float64       `json:"priceUSD"`
	ModalIDR          int64         `json:"modalIDR"`
	PriceIDR          int64         `json:"priceIDR"`
	ReferencePriceIDR int64         `json:"referencePriceIDR"`
	PromoPriceIDR     int64         `json:"promoPriceIDR"`
	HasPromo          bool          `json:"hasPromo"`
	DiscountPercent   int           `json:"discountPercent"`
	Tiers             []ProductTier `json:"tiers"`
	IsActive          bool          `json:"isActive"`
	IsForceOutOfStock bool          `json:"isForceOutOfStock"`
	IsCustomized      bool          `json:"isCustomized"`
	BaselineModalIDR  int64         `json:"baselineModalIDR"`
	IsAutoSuspended   bool          `json:"isAutoSuspended"`
	SurgePercent      float64       `json:"surgePercent"`
}

// ProductOverride represents stored admin custom settings for a product
type ProductOverride struct {
	ProductSlug       string    `json:"product_slug"`
	CustomName        string    `json:"custom_name"`
	CustomPriceIDR    int64     `json:"custom_price_idr"`
	PromoPriceIDR     int64     `json:"promo_price_idr"`
	CustomDescription string    `json:"custom_description"`
	CustomTerms       string    `json:"custom_terms"`
	IsActive          bool      `json:"is_active"`
	IsForceOutOfStock bool      `json:"is_force_out_of_stock"`
	BaselineModalIDR  int64     `json:"baseline_modal_idr"`
	IsAutoSuspended   bool      `json:"is_auto_suspended"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ProductOverrideRequest payload for updating per-product settings
type ProductOverrideRequest struct {
	CustomName        string `json:"custom_name"`
	CustomPriceIDR    int64  `json:"custom_price_idr"`
	PromoPriceIDR     int64  `json:"promo_price_idr"`
	CustomDescription string `json:"custom_description"`
	CustomTerms       string `json:"custom_terms"`
	IsActive          bool   `json:"is_active"`
	IsForceOutOfStock bool   `json:"is_force_out_of_stock"`
}

// AdminProductItem provides rich details for the admin products management table
type AdminProductItem struct {
	Slug                string        `json:"slug"`
	OriginalName        string        `json:"original_name"`
	CustomName          string        `json:"custom_name"`
	Name                string        `json:"name"`
	Type                string        `json:"type"`
	Logo                string        `json:"logo"`
	Region              string        `json:"region"`
	Stock               int           `json:"stock"`
	Warranty            string        `json:"warranty"`
	OriginalPriceUSD    float64       `json:"original_price_usd"`
	ModalIDR            int64         `json:"modal_idr"`
	EnvPriceIDR         int64         `json:"env_price_idr"`
	CalculatedPriceIDR  int64         `json:"calculated_price_idr"`
	CustomPriceIDR      int64         `json:"custom_price_idr"`
	ReferencePriceIDR   int64         `json:"reference_price_idr"`
	PromoPriceIDR       int64         `json:"promo_price_idr"`
	PriceIDR            int64         `json:"price_idr"`
	HasPromo            bool          `json:"has_promo"`
	DiscountPercent     int           `json:"discount_percent"`
	OriginalDescription string        `json:"original_description"`
	CustomDescription   string        `json:"custom_description"`
	Description         string        `json:"description"`
	CustomTerms         string        `json:"custom_terms"`
	Terms               string        `json:"terms"`
	IsActive            bool          `json:"is_active"`
	IsForceOutOfStock   bool          `json:"is_force_out_of_stock"`
	IsOriginalOutOfStock bool         `json:"is_original_out_of_stock"`
	IsCustomized        bool          `json:"is_customized"`
	BaselineModalIDR    int64         `json:"baseline_modal_idr"`
	IsAutoSuspended     bool          `json:"is_auto_suspended"`
	SurgePercent        float64       `json:"surge_percent"`
	Tiers               []ProductTier `json:"tiers"`
}

// CheckoutRequest payload from buyer
type CheckoutRequest struct {
	ProductSlug     string `json:"product_slug"`
	Quantity        int    `json:"quantity"`
	CustomerEmail   string `json:"customer_email"`
	CustomerContact string `json:"customer_contact"`
}

// CheckoutResponse returned to buyer
type CheckoutResponse struct {
	InvoiceNumber    string `json:"invoice_number"`
	ClaimCode        string `json:"claim_code"`
	ProductName      string `json:"product_name"`
	Quantity         int    `json:"quantity"`
	OriginalAmount   int64  `json:"original_amount"`
	UniqueCode       int    `json:"unique_code"`
	TotalAmount      int64  `json:"total_amount"`
	GatewayOrderID   string `json:"gateway_order_id"`
	QRURL            string `json:"qr_url"`
	ExpiresAt        string `json:"expires_at"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// OrderStatusResponse for payment page polling
type OrderStatusResponse struct {
	InvoiceNumber       string   `json:"invoice_number"`
	ClaimCode           string   `json:"claim_code"`
	ProductName         string   `json:"product_name"`
	ProductType         string   `json:"product_type"`
	Quantity            int      `json:"quantity"`
	TotalAmount         int64    `json:"total_amount"`
	PaymentStatus       string   `json:"payment_status"`
	FulfillmentStatus   string   `json:"fulfillment_status"`
	QRURL               string   `json:"qr_url"`
	DeliveredItems      []string `json:"delivered_items,omitempty"`
	WarrantyInfo        string   `json:"warranty_info"`
	CustomerContact     string   `json:"customer_contact"`
	CustomerEmail       string   `json:"customer_email"`
	PaidAt              string   `json:"paid_at,omitempty"`
	ExpiresAt           string   `json:"expires_at,omitempty"`
	IsQuotaOut          bool     `json:"is_quota_out,omitempty"`
	QuotaOutMessage     string   `json:"quota_out_message,omitempty"`
}

// ClaimResponse returned when querying a claim code
type ClaimResponse struct {
	Success            bool     `json:"success"`
	InvoiceNumber      string   `json:"invoice_number"`
	ClaimCode          string   `json:"claim_code"`
	ProductName        string   `json:"product_name"`
	ProductType        string   `json:"product_type"`
	Quantity           int      `json:"quantity"`
	PaymentStatus      string   `json:"payment_status"`
	FulfillmentStatus  string   `json:"fulfillment_status"`
	DeliveredItems     []string `json:"delivered_items,omitempty"`
	WarrantyInfo       string   `json:"warranty_info"`
	DigitalCoreViewURL string   `json:"digitalcore_view_url,omitempty"`
	CreatedAt          string   `json:"created_at"`
	IsQuotaOut         bool     `json:"is_quota_out,omitempty"`
	QuotaOutMessage    string   `json:"quota_out_message,omitempty"`
}

// AdminStats for admin dashboard analytics
type AdminStats struct {
	TotalRevenueIDR      int64   `json:"total_revenue_idr"`
	TotalOrders          int64   `json:"total_orders"`
	PaidOrders           int64   `json:"paid_orders"`
	PendingOrders        int64   `json:"pending_orders"`
	FailedOrders         int64   `json:"failed_orders"`
	DigitalCoreBalance   float64 `json:"digitalcore_balance"`
	DigitalCoreOrders    string  `json:"digitalcore_orders"`
	TodayRevenueIDR      int64   `json:"today_revenue_idr"`
	TodayOrders          int64   `json:"today_orders"`
}
