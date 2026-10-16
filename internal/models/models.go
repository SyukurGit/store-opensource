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
	PaymentGateway      string     `json:"payment_gateway"` // e.g. PAYMENTG, TRIPAY, MIDTRANS
	PaymentMethod       string     `json:"payment_method"`  // e.g. QRIS, VIRTUAL_ACCOUNT, EWALLET
	PaymentChannel      string     `json:"payment_channel"` // e.g. SHOPEEPAY, BCA, GOPAY
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
	Source            string        `json:"source,omitempty"` // "LOCAL" or "DIGITALCORE"
	SalesCount        int           `json:"salesCount"`
	DisplayOrder      int           `json:"displayOrder"`
	RatingAvg         float64       `json:"ratingAvg"`
	RatingCount       int           `json:"ratingCount"`

	// Multilingual fields (English)
	NameEN            string        `json:"name_en,omitempty"`
	TypeEN            string        `json:"type_en,omitempty"`
	RegionEN          string        `json:"region_en,omitempty"`
	WarrantyEN        string        `json:"warranty_en,omitempty"`
	DescriptionEN     string        `json:"description_en,omitempty"`
	TermsEN           string        `json:"terms_en,omitempty"`
}

// LocalProduct represents a standalone product managed directly in the local database
type LocalProduct struct {
	ID                int64     `json:"id"`
	Slug              string    `json:"slug"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	Logo              string    `json:"logo"`
	Region            string    `json:"region"`
	Warranty          string    `json:"warranty"`
	Description       string    `json:"description"`
	Terms             string    `json:"terms"`
	NameEN            string    `json:"name_en"`
	TypeEN            string    `json:"type_en"`
	RegionEN          string    `json:"region_en"`
	WarrantyEN        string    `json:"warranty_en"`
	DescriptionEN     string    `json:"description_en"`
	TermsEN           string    `json:"terms_en"`
	PriceIDR          int64     `json:"price_idr"`
	ReferencePriceIDR int64     `json:"reference_price_idr"`
	PromoPriceIDR     int64     `json:"promo_price_idr"`
	HasPromo          bool      `json:"has_promo"`
	DiscountPercent   int       `json:"discount_percent"`
	Stock             int       `json:"stock"`
	StockKeys         []string  `json:"stock_keys"` // In memory slice; stored as JSON text in DB
	DisplayOrder      int       `json:"display_order"`
	SalesCount        int       `json:"sales_count"`
	IsActive          bool      `json:"is_active"`
	IsForceOutOfStock bool      `json:"is_force_out_of_stock"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (lp *LocalProduct) ToProduct() *Product {
	sellPrice := lp.PriceIDR
	refPrice := lp.ReferencePriceIDR
	if refPrice <= 0 {
		refPrice = lp.PriceIDR
	}
	if lp.HasPromo && lp.PromoPriceIDR > 0 {
		sellPrice = lp.PromoPriceIDR
	}

	discPct := lp.DiscountPercent
	if discPct <= 0 && lp.HasPromo && refPrice > sellPrice {
		discPct = int(float64(refPrice-sellPrice) / float64(refPrice) * 100.0)
	}

	salesCount := lp.SalesCount
	if salesCount <= 0 {
		salesCount = 10
	}

	return &Product{
		Slug:              lp.Slug,
		Name:              lp.Name,
		Type:              lp.Type,
		Logo:              lp.Logo,
		Region:            lp.Region,
		Warranty:          lp.Warranty,
		Description:       lp.Description,
		Terms:             lp.Terms,
		PriceUSD:          0,
		ModalIDR:          0,
		PriceIDR:          sellPrice,
		ReferencePriceIDR: refPrice,
		PromoPriceIDR:     lp.PromoPriceIDR,
		HasPromo:          lp.HasPromo,
		DiscountPercent:   discPct,
		Stock:             lp.Stock,
		IsActive:          lp.IsActive,
		IsForceOutOfStock: lp.IsForceOutOfStock,
		IsCustomized:      true,
		Source:            "LOCAL",
		DisplayOrder:      lp.DisplayOrder,
		SalesCount:        salesCount,
		NameEN:            lp.NameEN,
		TypeEN:            lp.TypeEN,
		RegionEN:          lp.RegionEN,
		WarrantyEN:        lp.WarrantyEN,
		DescriptionEN:     lp.DescriptionEN,
		TermsEN:           lp.TermsEN,
	}
}

// LocalProductRequest represents the payload for creating or updating a local product
type LocalProductRequest struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	Logo              string   `json:"logo"`
	Region            string   `json:"region"`
	Warranty          string   `json:"warranty"`
	Description       string   `json:"description"`
	Terms             string   `json:"terms"`
	PriceIDR          int64    `json:"price_idr"`
	ReferencePriceIDR int64    `json:"reference_price_idr"`
	PromoPriceIDR     int64    `json:"promo_price_idr"`
	HasPromo          bool     `json:"has_promo"`
	StockKeys         []string `json:"stock_keys"` // Up to 10 keys
	DisplayOrder      int      `json:"display_order"`
	SalesCount        int      `json:"sales_count"`
	IsActive          bool     `json:"is_active"`
	IsForceOutOfStock bool     `json:"is_force_out_of_stock"`
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
	CustomLogo        string    `json:"custom_logo"`
	CustomNameEN      string    `json:"custom_name_en"`
	CustomTypeEN      string    `json:"custom_type_en"`
	CustomRegionEN    string    `json:"custom_region_en"`
	CustomWarrantyEN  string    `json:"custom_warranty_en"`
	CustomDescriptionEN string  `json:"custom_description_en"`
	CustomTermsEN     string    `json:"custom_terms_en"`
	DisplayOrder      int       `json:"display_order"`
	SalesCount        int       `json:"sales_count"`
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
	CustomLogo        string `json:"custom_logo"`
	DisplayOrder      int    `json:"display_order"`
	SalesCount        int    `json:"sales_count"`
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
	CustomLogo           string        `json:"custom_logo,omitempty"`
	Source               string        `json:"source"`
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
	CustomNameEN         string        `json:"custom_name_en,omitempty"`
	CustomTypeEN         string        `json:"custom_type_en,omitempty"`
	CustomRegionEN       string        `json:"custom_region_en,omitempty"`
	CustomWarrantyEN     string        `json:"custom_warranty_en,omitempty"`
	CustomDescriptionEN  string        `json:"custom_description_en,omitempty"`
	CustomTermsEN        string        `json:"custom_terms_en,omitempty"`
	DisplayOrder         int           `json:"display_order"`
	SalesCount           int           `json:"sales_count"`
	IsActive             bool          `json:"is_active"`
	IsForceOutOfStock    bool          `json:"is_force_out_of_stock"`
	IsOriginalOutOfStock bool          `json:"is_original_out_of_stock"`
	IsCustomized         bool          `json:"is_customized"`
	BaselineModalIDR     int64         `json:"baseline_modal_idr"`
	IsAutoSuspended      bool          `json:"is_auto_suspended"`
	SurgePercent         float64       `json:"surge_percent"`
	Tiers                []ProductTier `json:"tiers"`
}

// AdminEtalaseProductItem provides a unified model for the /admin/reviews etalase table
type AdminEtalaseProductItem struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	Type         string `json:"type"`
	Logo         string `json:"logo"`
	Region       string `json:"region"`
	IsActive     bool   `json:"is_active"`
	DisplayOrder int    `json:"display_order"`
	SalesCount   int    `json:"sales_count"`
	Source       string `json:"source"` // "LOCAL" or "DIGITALCORE"
}

// CheckoutRequest payload from buyer
type CheckoutRequest struct {
	ProductSlug     string `json:"product_slug"`
	Quantity        int    `json:"quantity"`
	CustomerEmail   string `json:"customer_email"`
	CustomerContact string `json:"customer_contact"`
	VoucherCode     string `json:"voucher_code,omitempty"`
	PaymentGateway  string `json:"payment_gateway,omitempty"`
	PaymentMethod   string `json:"payment_method,omitempty"`
	PaymentChannel  string `json:"payment_channel,omitempty"`
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
	PaymentGateway   string `json:"payment_gateway,omitempty"`
	PaymentMethod    string `json:"payment_method,omitempty"`
	PaymentChannel   string `json:"payment_channel,omitempty"`
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
	PaymentGateway      string   `json:"payment_gateway,omitempty"`
	PaymentMethod       string   `json:"payment_method,omitempty"`
	PaymentChannel      string   `json:"payment_channel,omitempty"`
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
	SilvexBalance        float64 `json:"silvex_balance"`
	SilvexConnected      bool    `json:"silvex_connected"`
	SilvexError          string  `json:"silvex_error,omitempty"`
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

// ProductReview represents buyer or admin review
type ProductReview struct {
	ID           int64     `json:"id"`
	ProductSlug  string    `json:"product_slug"`
	OrderInvoice string    `json:"order_invoice,omitempty"`
	AuthorName   string    `json:"author_name"`
	Rating       int       `json:"rating"`
	ReviewText   string    `json:"review_text"`
	IsVisible    bool      `json:"is_visible"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ReviewStats aggregates rating score and count
type ReviewStats struct {
	AverageRating float64     `json:"average_rating"`
	TotalReviews  int         `json:"total_reviews"`
	Distribution  map[int]int `json:"distribution"`
}

// ProductDisplayStatsRequest payload for updating priority order and sold count
type ProductDisplayStatsRequest struct {
	DisplayOrder int `json:"display_order"`
	SalesCount   int `json:"sales_count"`
}

// SubmitReviewRequest payload from buyer on claim page
type SubmitReviewRequest struct {
	InvoiceNumber string `json:"invoice_number"`
	ClaimCode     string `json:"claim_code"`
	Rating        int    `json:"rating"`
	ReviewText    string `json:"review_text"`
	AuthorName    string `json:"author_name,omitempty"`
}

// AdminReviewRequest payload for admin creating/editing reviews
type AdminReviewRequest struct {
	ProductSlug  string `json:"product_slug"`
	AuthorName   string `json:"author_name"`
	Rating       int    `json:"rating"`
	ReviewText   string `json:"review_text"`
	IsVisible    bool   `json:"is_visible"`
	OrderInvoice string `json:"order_invoice,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// ProductTranslationItem represents product translation info for dedicated Admin Translations menu
type ProductTranslationItem struct {
	Slug          string `json:"slug"`
	Source        string `json:"source"` // "LOCAL" or "DIGITALCORE"
	NameID        string `json:"name_id"`
	TypeID        string `json:"type_id"`
	RegionID      string `json:"region_id"`
	WarrantyID    string `json:"warranty_id"`
	DescriptionID string `json:"description_id"`
	TermsID       string `json:"terms_id"`

	NameEN        string `json:"name_en"`
	TypeEN        string `json:"type_en"`
	RegionEN      string `json:"region_en"`
	WarrantyEN    string `json:"warranty_en"`
	DescriptionEN string `json:"description_en"`
	TermsEN       string `json:"terms_en"`

	IsTranslated  bool   `json:"is_translated"`
	IsActive      bool   `json:"is_active"`
}

// UpdateTranslationRequest represents request body to update product EN translations
type UpdateTranslationRequest struct {
	NameEN        string `json:"name_en"`
	TypeEN        string `json:"type_en"`
	RegionEN      string `json:"region_en"`
	WarrantyEN    string `json:"warranty_en"`
	DescriptionEN string `json:"description_en"`
	TermsEN       string `json:"terms_en"`
}

// AutoDraftRequest represents payload sent to auto-draft translation endpoint
type AutoDraftRequest struct {
	Texts map[string]string `json:"texts"` // e.g. {"name": "...", "description": "..."}
}



