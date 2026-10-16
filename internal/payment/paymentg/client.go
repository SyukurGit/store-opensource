package paymentg

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

// Client handles low-level HTTP communication with the PaymentG QRIS API server
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient creates a new PaymentG HTTP client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// CreatePaymentGOrderRequest defines the JSON payload for PaymentG order creation
type CreatePaymentGOrderRequest struct {
	ReferenceID   string `json:"reference_id"`
	Amount        int64  `json:"amount"`
	ExpiryMinutes int    `json:"expiry_minutes"`
	Metadata      string `json:"metadata,omitempty"`
}

// OrderData represents the order details returned by PaymentG
type OrderData struct {
	OrderID          string `json:"order_id"`
	ReferenceID      string `json:"reference_id"`
	OriginalAmount   int64  `json:"original_amount"`
	UniqueCode       int    `json:"unique_code"`
	TotalAmount      int64  `json:"total_amount"`
	Status           string `json:"status"`
	QRURL            string `json:"qr_url"`
	ExpiresAt        string `json:"expires_at"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	PaidAt           string `json:"paid_at,omitempty"`
	ShopeeTxID       string `json:"shopee_tx_id,omitempty"`
}

// APIResponse represents the standard JSON wrapper from PaymentG
type APIResponse struct {
	Success bool       `json:"success"`
	Data    *OrderData `json:"data"`
	Error   string     `json:"error,omitempty"`
	Message string     `json:"message,omitempty"`
}

// CreateQRISOrder creates a dynamic QRIS order on PaymentG
func (c *Client) CreateQRISOrder(ctx context.Context, invoice string, amount int64, metadata string) (*OrderData, error) {
	apiURL := fmt.Sprintf("%s/orders", strings.TrimRight(c.cfg.PaymentGAPIURL, "/"))

	reqBody := CreatePaymentGOrderRequest{
		ReferenceID:   invoice,
		Amount:        amount,
		ExpiryMinutes: 15,
		Metadata:      metadata,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PaymentG request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.cfg.PaymentGAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call PaymentG API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PaymentG response: %w", err)
	}

	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse PaymentG response: %w (body: %s)", err, string(body))
	}

	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) || !res.Success || res.Data == nil {
		errMsg := res.Error
		if errMsg == "" {
			errMsg = res.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("status code %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("PaymentG order creation failed: %s", errMsg)
	}

	// Resolve full QR URL if relative
	res.Data.QRURL = c.ResolveFullQRURL(res.Data.QRURL)

	return res.Data, nil
}

// CheckOrderStatus queries PaymentG order status
func (c *Client) CheckOrderStatus(ctx context.Context, gatewayOrderID string) (*OrderData, error) {
	apiURL := fmt.Sprintf("%s/orders/%s", strings.TrimRight(c.cfg.PaymentGAPIURL, "/"), url.PathEscape(gatewayOrderID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.cfg.PaymentGAPIKey)

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
		return nil, fmt.Errorf("failed to get order status: %s", res.Error)
	}

	res.Data.QRURL = c.ResolveFullQRURL(res.Data.QRURL)
	return res.Data, nil
}

// TriggerInstantMutationCheck triggers an immediate ShopeePay mutation check ("Saya Sudah Bayar")
func (c *Client) TriggerInstantMutationCheck(ctx context.Context, gatewayOrderID string) error {
	apiURL := fmt.Sprintf("%s/orders/%s/check", strings.TrimRight(c.cfg.PaymentGAPIURL, "/"), url.PathEscape(gatewayOrderID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.cfg.PaymentGAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mutation check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ResolveFullQRURL converts relative QR URL to absolute URL
func (c *Client) ResolveFullQRURL(rawURL string) string {
	if rawURL == "" || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	u, err := url.Parse(c.cfg.PaymentGAPIURL)
	if err != nil {
		return rawURL
	}

	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, rawURL)
}
