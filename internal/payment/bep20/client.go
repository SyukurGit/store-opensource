package bep20

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"storeku/internal/config"
)

// OrderData matches the response structure from PaymentG (USDT BEP-20)
type OrderData struct {
	OrderID          string  `json:"order_id"`
	ReferenceID      string  `json:"reference_id"`
	Network          string  `json:"network"`
	ChainID          int     `json:"chain_id"`
	Currency         string  `json:"currency"`
	TokenContract    string  `json:"token_contract"`
	WalletAddress    string  `json:"wallet_address"`
	OriginalAmount   float64 `json:"original_amount"`
	UniqueCode       int     `json:"unique_code"`
	TotalAmount      float64 `json:"total_amount"`
	TotalAmountStr   string  `json:"total_amount_str"`
	CexWarning       string  `json:"cex_warning"`
	Status           string  `json:"status"`
	QRURL            string  `json:"qr_url"`
	ExpiresAt        string  `json:"expires_at"`
	ExpiresInSeconds int     `json:"expires_in_seconds"`
	PaidAt           string  `json:"paid_at,omitempty"`
	TxHash           string  `json:"tx_hash,omitempty"`
}

// CheckStatusData matches the response structure from POST /api/orders/:id/check
type CheckStatusData struct {
	OrderID            string `json:"order_id"`
	Status             string `json:"status"`
	TotalAmountStr     string `json:"total_amount_str"`
	TxHash             string `json:"tx_hash"`
	PaidAt             string `json:"paid_at"`
	ScannedNow         bool   `json:"scanned_now"`
	NextCheckInSeconds int    `json:"next_check_in_seconds"`
}

// APIResponse represents generic PaymentG BEP20 order creation response
type APIResponse struct {
	Success bool       `json:"success"`
	Data    *OrderData `json:"data"`
	Error   string     `json:"error,omitempty"`
	Message string     `json:"message,omitempty"`
}

// CheckAPIResponse represents generic PaymentG BEP20 order status check response
type CheckAPIResponse struct {
	Success bool             `json:"success"`
	Data    *CheckStatusData `json:"data"`
	Error   string           `json:"error,omitempty"`
	Message string           `json:"message,omitempty"`
}

// Client communicates with PaymentG USDT BEP-20 HTTP endpoints
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient initializes a new BEP20 client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) getBaseURL() string {
	u := strings.TrimRight(c.cfg.PaymentGBEP20APIURL, "/")
	if u == "" {
		u = "http://localhost:3400"
	}
	// Strip trailing /orders if user included it in env
	u = strings.TrimSuffix(u, "/orders")

	// If no path like /api or /api/sandbox is present, default to /api
	if !strings.Contains(u, "/api") {
		u = u + "/api"
	}
	return u
}

func (c *Client) getAPIKey() string {
	return c.cfg.PaymentGBEP20APIKey
}

// CreateOrderRequest is the payload sent to create a new USDT BEP-20 order
type CreateOrderRequest struct {
	ReferenceID string  `json:"reference_id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
}

// CreateBEP20Order creates a new USDT BEP-20 bill on PaymentG
func (c *Client) CreateBEP20Order(ctx context.Context, invoice string, amountUSDT float64) (*OrderData, error) {
	if amountUSDT < 0.01 {
		amountUSDT = 0.01
	}

	apiURL := fmt.Sprintf("%s/orders", c.getBaseURL())
	reqBody := CreateOrderRequest{
		ReferenceID: invoice,
		Amount:      amountUSDT,
		Currency:    "USDT",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	apiKey := c.getAPIKey()
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call PaymentG BEP20: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse PaymentG BEP20 response: %w (body: %s)", err, string(body))
	}

	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) || !res.Success || res.Data == nil {
		errMsg := res.Error
		if errMsg == "" {
			errMsg = res.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("PaymentG BEP20 order creation failed: %s", errMsg)
	}

	// Resolve full QR URL if relative
	res.Data.QRURL = c.ResolveFullQRURL(res.Data.QRURL)

	return res.Data, nil
}

// CheckOrderStatus queries status and triggers on-chain scan of an order on PaymentG BEP20
func (c *Client) CheckOrderStatus(ctx context.Context, orderID string) (*CheckStatusData, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, fmt.Errorf("order ID cannot be empty")
	}

	apiURL := fmt.Sprintf("%s/orders/%s/check", c.getBaseURL(), url.PathEscape(orderID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return nil, err
	}

	apiKey := c.getAPIKey()
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query PaymentG BEP20 status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}

	var res CheckAPIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w (body: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK || !res.Success || res.Data == nil {
		errMsg := res.Error
		if errMsg == "" {
			errMsg = res.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("PaymentG BEP20 status query failed: %s", errMsg)
	}

	return res.Data, nil
}

// ResolveFullQRURL converts a relative QR image path to a full HTTP URL
func (c *Client) ResolveFullQRURL(rawQR string) string {
	rawQR = strings.TrimSpace(rawQR)
	if rawQR == "" {
		return ""
	}
	if strings.HasPrefix(rawQR, "http://") || strings.HasPrefix(rawQR, "https://") || strings.HasPrefix(rawQR, "data:image/") {
		return rawQR
	}

	base := c.getBaseURL()
	parsedBase, err := url.Parse(base)
	if err != nil {
		return rawQR
	}

	if strings.HasPrefix(rawQR, "/") {
		return fmt.Sprintf("%s://%s%s", parsedBase.Scheme, parsedBase.Host, rawQR)
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), rawQR)
}
