package provider

// FulfillmentRequest contains the normalized payload required to fulfill an order
// regardless of whether it's local inventory or an external API vendor.
type FulfillmentRequest struct {
	InvoiceNumber   string
	ProductSlug     string
	ProviderItemID  string // Upstream SKU / Raw ID / Slug
	Quantity        int
	CustomerEmail   string
	CustomerContact string
	TotalAmountIDR  int64
	Metadata        map[string]interface{}
}

// FulfillmentResult contains the normalized result returned by a provider fulfillment attempt.
type FulfillmentResult struct {
	Success          bool
	ProviderOrderID  string   // Upstream vendor order ID or "LOCAL-ORDER"
	ProviderViewURL  string   // URL to view/claim license on upstream vendor portal
	DeliveredItems   []string // Array of serial keys / credentials / instructions
	WholesaleCostUSD float64  // Actual wholesale cost charged (in USD, or 0 if IDR native)
	WholesaleCostIDR int64    // Actual wholesale cost charged (in IDR)
	ErrorMessage     string
	RawResponse      string
}

// HealthResult contains diagnostic and connectivity information for a provider.
type HealthResult struct {
	Success      bool    `json:"success"`
	Configured   bool    `json:"configured"`
	ProviderID   string  `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	Balance      float64 `json:"balance"`
	Currency     string  `json:"currency"` // "USD" or "IDR"
	TotalOrders  string  `json:"total_orders"`
	HTTPStatus   int     `json:"http_status"`
	Message      string  `json:"message"`
	RawResponse  string  `json:"raw_response,omitempty"`
}
