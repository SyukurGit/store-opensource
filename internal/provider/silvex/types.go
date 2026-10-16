package silvex

// DeliveredItem represents each delivered license or credential object returned by Silvex AI.
type DeliveredItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// OrderRequest is the JSON payload sent to POST /api/public/reseller/order.
type OrderRequest struct {
	ProductID string `json:"productId"`
	Qty       int    `json:"qty"`
}

// OrderResponse is the JSON response from POST /api/public/reseller/order.
type OrderResponse struct {
	OK        bool            `json:"ok"`
	OrderID   string          `json:"orderId"`
	Status    string          `json:"status"` // "Completed", "Pending"
	Total     float64         `json:"total"`
	Balance   float64         `json:"balance"`
	Delivered []DeliveredItem `json:"delivered"`
	Error     string          `json:"error,omitempty"`
}

// BalanceResponse is the JSON response from GET /api/public/reseller/balance.
type BalanceResponse struct {
	OK      bool    `json:"ok"`
	Balance float64 `json:"balance"`
	Error   string  `json:"error,omitempty"`
}

// MeResponse is the JSON response from GET /api/public/reseller/me.
type MeResponse struct {
	OK      bool    `json:"ok"`
	Balance float64 `json:"balance"`
	Email   string  `json:"email,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// RawProduct matches products returned by GET /api/public/reseller/products.
// Uses flexible tag matching to accommodate various field name formats.
type RawProduct struct {
	ID          interface{} `json:"id"`
	ProductID   string      `json:"productId,omitempty"`
	Name        string      `json:"name"`
	Title       string      `json:"title,omitempty"`
	Category    string      `json:"category,omitempty"`
	Type        string      `json:"type,omitempty"`
	Price       float64     `json:"price"`
	PriceUSD    float64     `json:"priceUSD,omitempty"`
	Stock       int         `json:"stock"`
	Description string      `json:"description,omitempty"`
	Desc        string      `json:"desc,omitempty"`
	Logo        string      `json:"logo,omitempty"`
	Warranty    string      `json:"warranty,omitempty"`
	Status      string      `json:"status,omitempty"`
}
