package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"storeku/internal/config"
)

type PaymentGClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewPaymentGClient(cfg *config.Config) *PaymentGClient {
	return &PaymentGClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type CreatePaymentGOrderRequest struct {
	ReferenceID   string `json:"reference_id"`
	Amount        int64  `json:"amount"`
	ExpiryMinutes int    `json:"expiry_minutes"`
	Metadata      string `json:"metadata,omitempty"`
}

type PaymentGOrderData struct {
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

type PaymentGResponse struct {
	Success bool               `json:"success"`
	Data    *PaymentGOrderData `json:"data"`
	Error   string             `json:"error,omitempty"`
	Message string             `json:"message,omitempty"`
}

// CreateQRISOrder creates a dynamic QRIS order with PaymentG
func (c *PaymentGClient) CreateQRISOrder(invoice string, amount int64, metadata string) (*PaymentGOrderData, error) {
	apiURL := fmt.Sprintf("%s/orders", strings.TrimRight(c.cfg.PaymentGAPIURL, "/"))

	reqBody := CreatePaymentGOrderRequest{
		ReferenceID:   invoice,
		Amount:        amount,
		ExpiryMinutes: 15,
		Metadata:      metadata,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBuffer(bodyBytes))
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

	var res PaymentGResponse
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

// CheckOrderStatus checks the status of an order on PaymentG
func (c *PaymentGClient) CheckOrderStatus(gatewayOrderID string) (*PaymentGOrderData, error) {
	apiURL := fmt.Sprintf("%s/orders/%s", strings.TrimRight(c.cfg.PaymentGAPIURL, "/"), url.PathEscape(gatewayOrderID))

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.cfg.PaymentGAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res PaymentGResponse
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
func (c *PaymentGClient) TriggerInstantMutationCheck(gatewayOrderID string) error {
	apiURL := fmt.Sprintf("%s/orders/%s/check", strings.TrimRight(c.cfg.PaymentGAPIURL, "/"), url.PathEscape(gatewayOrderID))

	req, err := http.NewRequest(http.MethodPost, apiURL, nil)
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
func (c *PaymentGClient) ResolveFullQRURL(rawURL string) string {
	if rawURL == "" || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	u, err := url.Parse(c.cfg.PaymentGAPIURL)
	if err != nil {
		return rawURL
	}

	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, rawURL)
}
