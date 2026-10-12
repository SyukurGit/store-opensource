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
	VoucherCode         string     `json:"voucher_code,omitempty"`
	DiscountIDR         int64      `json:"discount_idr"`
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
	CustomType        string        `json:"customType,omitempty"`
	Logo              string        `json:"logo"`
	Region            string        `json:"region"`
	CustomRegion      string        `json:"customRegion,omitempty"`
	Stock             int           `json:"stock"`
	CustomStock       int           `json:"customStock"`
	StockCapped       bool          `json:"stockCapped"`
	Warranty          string        `json:"warranty"`
	CustomWarranty    string        `json:"customWarranty,omitempty"`
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
	CustomType        string    `json:"custom_type"`
	CustomRegion      string    `json:"custom_region"`
	CustomWarranty    string    `json:"custom_warranty"`
	CustomStock       int       `json:"custom_stock"`
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
	CustomType        string `json:"custom_type"`
	CustomRegion      string `json:"custom_region"`
	CustomWarranty    string `json:"custom_warranty"`
	CustomStock       int    `json:"custom_stock"`
	CustomPriceIDR    int64  `json:"custom_price_idr"`
	PromoPriceIDR     int64  `json:"promo_price_idr"`
	CustomDescription string `json:"custom_description"`
	CustomTerms       string `json:"custom_terms"`
	IsActive          bool   `json:"is_active"`
	IsForceOutOfStock bool   `json:"is_force_out_of_stock"`
}

// AdminProductItem provides rich details for the admin products management table
type AdminProductItem struct {
	Slug                 string        `json:"slug"`
	OriginalName         string        `json:"original_name"`
	CustomName           string        `json:"custom_name"`
	Name                 string        `json:"name"`
	OriginalType         string        `json:"original_type"`
	CustomType           string        `json:"custom_type"`
	Type                 string        `json:"type"`
	Logo                 string        `json:"logo"`
	OriginalRegion       string        `json:"original_region"`
	CustomRegion         string        `json:"custom_region"`
	Region               string        `json:"region"`
	OriginalStock        int           `json:"original_stock"`
	CustomStock          int           `json:"custom_stock"`
	Stock                int           `json:"stock"`
	OriginalWarranty     string        `json:"original_warranty"`
	CustomWarranty       string        `json:"custom_warranty"`
	Warranty             string        `json:"warranty"`
	OriginalPriceUSD     float64       `json:"original_price_usd"`
	ModalIDR             int64         `json:"modal_idr"`
	EnvPriceIDR          int64         `json:"env_price_idr"`
	CalculatedPriceIDR   int64         `json:"calculated_price_idr"`
	CustomPriceIDR       int64         `json:"custom_price_idr"`
	ReferencePriceIDR    int64         `json:"reference_price_idr"`
	PromoPriceIDR        int64         `json:"promo_price_idr"`
	PriceIDR             int64         `json:"price_idr"`
	HasPromo             bool          `json:"has_promo"`
	DiscountPercent      int           `json:"discount_percent"`
	OriginalDescription  string        `json:"original_description"`
	CustomDescription    string        `json:"custom_description"`
	Description          string        `json:"description"`
	CustomTerms          string        `json:"custom_terms"`
	Terms                string        `json:"terms"`
	IsActive             bool          `json:"is_active"`
	IsForceOutOfStock    bool          `json:"is_force_out_of_stock"`
	IsOriginalOutOfStock bool          `json:"is_original_out_of_stock"`
	IsCustomized         bool          `json:"is_customized"`
	BaselineModalIDR     int64         `json:"baseline_modal_idr"`
	IsAutoSuspended      bool          `json:"is_auto_suspended"`
	SurgePercent         float64       `json:"surge_percent"`
	Tiers                []ProductTier `json:"tiers"`
}

// CheckoutRequest payload from buyer
type CheckoutRequest struct {
	ProductSlug     string `json:"product_slug"`
	Quantity        int    `json:"quantity"`
	CustomerEmail   string `json:"customer_email"`
	CustomerContact string `json:"customer_contact"`
	VoucherCode     string `json:"voucher_code,omitempty"`
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
	VoucherCode      string `json:"voucher_code,omitempty"`
	DiscountIDR      int64  `json:"discount_idr,omitempty"`
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
	DigitalCoreConnected bool    `json:"digitalcore_connected"`
	DigitalCoreError     string  `json:"digitalcore_error,omitempty"`
	TodayRevenueIDR      int64   `json:"today_revenue_idr"`
	TodayOrders          int64   `json:"today_orders"`
}

// DigitalCoreCheckResult provides health & diagnostic information about upstream DigitalCore connection
type DigitalCoreCheckResult struct {
	Success      bool    `json:"success"`
	Configured   bool    `json:"configured"`
	BaseURL      string  `json:"base_url"`
	APIKeyMasked string  `json:"api_key_masked"`
	HTTPStatus   int     `json:"http_status"`
	Balance      float64 `json:"balance"`
	TotalOrders  string  `json:"total_orders"`
	RawResponse  string  `json:"raw_response,omitempty"`
	Message      string  `json:"message"`
}

// Voucher represents a promo code with cryptographic signature and usage limits
type Voucher struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	DiscountIDR int64      `json:"discount_idr"`
	MinSpendIDR int64      `json:"min_spend_idr"`
	MaxUsage    int        `json:"max_usage"`
	UsedCount   int        `json:"used_count"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `json:"is_active"`
	Signature   string     `json:"signature"`
	Note        string     `json:"note"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateVoucherRequest payload for creating a new voucher in admin
type CreateVoucherRequest struct {
	Code          string `json:"code"`
	DiscountIDR   int64  `json:"discount_idr"`
	MinSpendIDR   int64  `json:"min_spend_idr"`
	MaxUsage      int    `json:"max_usage"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	Note          string `json:"note,omitempty"`
	AdminPassword string `json:"admin_password"`
}

// ValidateVoucherRequest payload from frontend checkout
type ValidateVoucherRequest struct {
	Code        string `json:"code"`
	ProductSlug string `json:"product_slug"`
	Quantity    int    `json:"quantity"`
}

// ValidateVoucherResponse returned to frontend checkout
type ValidateVoucherResponse struct {
	Valid             bool   `json:"valid"`
	Code              string `json:"code"`
	DiscountIDR       int64  `json:"discount_idr"`
	DiscountFormatted string `json:"discount_formatted"`
	SubtotalOriginal  int64  `json:"subtotal_original"`
	SubtotalFinal     int64  `json:"subtotal_final"`
	Message           string `json:"message"`
}

// DatabaseStats holds aggregate counts and storage metrics for admin maintenance
type DatabaseStats struct {
	DatabaseSizeBytes        int64  `json:"database_size_bytes"`
	DatabaseSizeFormatted    string `json:"database_size_formatted"`
	TotalOrders              int64  `json:"total_orders"`
	PaidOrders               int64  `json:"paid_orders"`
	PendingOrders            int64  `json:"pending_orders"`
	ExpiredCancelledOrders   int64  `json:"expired_cancelled_orders"`
	OrdersOlderThan3Days     int64  `json:"orders_older_than_3_days"`
	OrdersOlderThan7Days     int64  `json:"orders_older_than_7_days"`
	OrdersOlderThan30Days    int64  `json:"orders_older_than_30_days"`
	OrdersWithKeysOlder3Days int64  `json:"orders_with_keys_older_3_days"`
	TotalLoginLogs           int64  `json:"total_login_logs"`
	FailedLoginLogs          int64  `json:"failed_login_logs"`
	TotalVouchers            int64  `json:"total_vouchers"`
	ExpiredUsedVouchers      int64  `json:"expired_used_vouchers"`
	CachedProductsCount      int64  `json:"cached_products_count"`
}

// DatabaseCleanRequest payload for selective database cleanup
type DatabaseCleanRequest struct {
	AdminPassword       string `json:"admin_password"`
	CleanOrders         bool   `json:"clean_orders"`
	OrdersMode          string `json:"orders_mode"` // "ALL", "EXPIRED_CANCELLED", "OLDER_THAN"
	OrdersOlderDays     int    `json:"orders_older_days"`
	CleanDeliveredKeys  bool   `json:"clean_delivered_keys"`
	KeysOlderDays       int    `json:"keys_older_days"`
	CleanLoginLogs      bool   `json:"clean_login_logs"`
	LogsMode            string `json:"logs_mode"` // "ALL", "FAILED_ONLY", "OLDER_THAN_7_DAYS", "OLDER_THAN_30_DAYS"
	CleanVouchers       bool   `json:"clean_vouchers"`
	VouchersMode        string `json:"vouchers_mode"` // "EXPIRED_USED", "ALL"
	CleanCachedProducts bool   `json:"clean_cached_products"`
	RunVacuum           bool   `json:"run_vacuum"`
}

// DatabaseCleanResult summary returned after database cleanup execution
type DatabaseCleanResult struct {
	OrdersDeleted      int64  `json:"orders_deleted"`
	KeysCleaned        int64  `json:"keys_cleaned"`
	LogsDeleted        int64  `json:"logs_deleted"`
	VouchersDeleted    int64  `json:"vouchers_deleted"`
	CachedCleaned      int64  `json:"cached_cleaned"`
	VacuumExecuted     bool   `json:"vacuum_executed"`
	DatabaseSizeBefore string `json:"database_size_before"`
	DatabaseSizeAfter  string `json:"database_size_after"`
	Message            string `json:"message"`
}

