package binance

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

// OrderData matches the response structure from PaymentG (Binance Pay Edition)
type OrderData struct {
	OrderID            string      `json:"order_id"`
	ReferenceID        string      `json:"reference_id"`
	Currency           string      `json:"currency"`
	OriginalAmount     float64     `json:"original_amount"`
	UniqueCode         int         `json:"unique_code"`
	TotalAmount        float64     `json:"total_amount"`
	TotalAmountStr     string      `json:"total_amount_str"`
	BinancePayID       string      `json:"binance_pay_id"`
	BinanceAccountName string      `json:"binance_account_name"`
	BinanceUniQRURL    string      `json:"binance_uni_qr_url"`
	Status             string      `json:"status"`
	QRURL              string      `json:"qr_url"`
	ExpiresAt          string      `json:"expires_at"`
	ExpiresInSeconds   int         `json:"expires_in_seconds"`
	PaidAt             string      `json:"paid_at,omitempty"`
	BinanceTxID        string      `json:"binance_tx_id,omitempty"`
}

// APIResponse represents generic PaymentG Binance JSON response
type APIResponse struct {
	Success bool       `json:"success"`
	Data    *OrderData `json:"data"`
	Error   string     `json:"error,omitempty"`
	Message string     `json:"message,omitempty"`
}

// Client communicates with PaymentG Binance Edition HTTP endpoints
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient initializes a new Binance client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) getBaseURL() string {
	u := strings.TrimRight(c.cfg.PaymentGBinanceAPIURL, "/")
	if u == "" {
		// Fallback to PAYMENTG_API_URL or default local sandbox port 3300
		if c.cfg.PaymentGAPIURL != "" && strings.Contains(c.cfg.PaymentGAPIURL, "3300") {
			u = strings.TrimRight(c.cfg.PaymentGAPIURL, "/")
		} else {
			u = "http://localhost:3300/api/sandbox"
		}
	}
	return u
}

func (c *Client) getAPIKey() string {
	if c.cfg.PaymentGBinanceAPIKey != "" {
		return c.cfg.PaymentGBinanceAPIKey
	}
	return c.cfg.PaymentGAPIKey
}

type CreateOrderRequest struct {
	ReferenceID   string  `json:"reference_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	ExpiryMinutes int     `json:"expiry_minutes"`
}

// CreateBinanceOrder creates a new crypto order on PaymentG Binance
func (c *Client) CreateBinanceOrder(ctx context.Context, invoice string, amountUSDT float64, expiryMinutes int) (*OrderData, error) {
	if amountUSDT < 0.01 {
		amountUSDT = 0.01 // minimum Binance transfer rule
	}
	if expiryMinutes <= 0 {
		expiryMinutes = 15
	}

	apiURL := fmt.Sprintf("%s/orders", c.getBaseURL())
	reqBody := CreateOrderRequest{
		ReferenceID:   invoice,
		Amount:        amountUSDT,
		Currency:      "USDT",
		ExpiryMinutes: expiryMinutes,
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
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call PaymentG Binance: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse Binance response: %w (body: %s)", err, string(body))
	}

	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) || !res.Success || res.Data == nil {
		errMsg := res.Error
		if errMsg == "" {
			errMsg = res.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("Binance order creation failed: %s", errMsg)
	}

	// Resolve full QR URL if relative
	res.Data.QRURL = c.ResolveFullQRURL(res.Data.QRURL)

	return res.Data, nil
}

// CheckOrderStatus queries order status from PaymentG Binance
func (c *Client) CheckOrderStatus(ctx context.Context, gatewayOrderID string) (*OrderData, error) {
	apiURL := fmt.Sprintf("%s/orders/%s", c.getBaseURL(), url.PathEscape(gatewayOrderID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	apiKey := c.getAPIKey()
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if !res.Success || res.Data == nil {
		return nil, fmt.Errorf("failed to query Binance order status: %s", res.Error)
	}

	res.Data.QRURL = c.ResolveFullQRURL(res.Data.QRURL)
	return res.Data, nil
}

// TriggerInstantMutationCheck triggers on-demand instant check on PaymentG Binance ("Saya Sudah Bayar")
func (c *Client) TriggerInstantMutationCheck(ctx context.Context, gatewayOrderID string) (*OrderData, error) {
	apiURL := fmt.Sprintf("%s/orders/%s/check", c.getBaseURL(), url.PathEscape(gatewayOrderID))

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
		return nil, err
	}
	defer resp.Body.Close()

	var res APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if !res.Success || res.Data == nil {
		return nil, fmt.Errorf("instant check failed: %s", res.Error)
	}

	res.Data.QRURL = c.ResolveFullQRURL(res.Data.QRURL)
	return res.Data, nil
}

// ResolveFullQRURL converts relative QR URL to absolute URL
func (c *Client) ResolveFullQRURL(rawURL string) string {
	if rawURL == "" || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	u, err := url.Parse(c.getBaseURL())
	if err != nil {
		return rawURL
	}

	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, rawURL)
}

func (c *Client) Config() *config.Config {
	return c.cfg
}
