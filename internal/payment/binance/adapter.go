package binance

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"storeku/internal/config"
	"storeku/internal/payment"
)

// Adapter adapts PaymentG (Binance Pay Edition) to the unified payment.Gateway interface
type Adapter struct {
	cfg    *config.Config
	client *Client
}

// NewAdapter creates a new Binance Pay adapter
func NewAdapter(cfg *config.Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: NewClient(cfg),
	}
}

// NewAdapterWithClient allows injecting a custom client (e.g. for testing)
func NewAdapterWithClient(cfg *config.Config, client *Client) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: client,
	}
}

// ID returns the unique gateway identifier
func (a *Adapter) ID() string {
	return "BINANCE"
}

// Name returns the human-readable display name
func (a *Adapter) Name() string {
	return "Binance Pay (USDT)"
}

// SupportedMethods returns supported payment methods
func (a *Adapter) SupportedMethods() []payment.Method {
	return []payment.Method{payment.MethodBinancePay, payment.MethodCrypto}
}

// CreatePayment creates a Binance Pay crypto order
func (a *Adapter) CreatePayment(ctx context.Context, req *payment.CreatePaymentRequest) (*payment.PaymentResponse, error) {
	amountUSDT := req.AmountCrypto
	if amountUSDT <= 0 && req.Amount > 0 {
		rate := a.cfg.USDToIDRRate
		if rate <= 0 {
			rate = 18000.0
		}
		// Convert IDR to USD rounded to 2 decimal places
		amountUSDT = math.Round((float64(req.Amount)/rate)*100.0) / 100.0
	}
	if amountUSDT < 0.01 {
		amountUSDT = 0.01
	}

	expiryMin := req.ExpiryMinutes
	if expiryMin <= 0 {
		expiryMin = 15
	}

	orderData, err := a.client.CreateBinanceOrder(ctx, req.InvoiceNumber, amountUSDT, expiryMin)
	if err != nil {
		return nil, err
	}

	var expiresAtTime *time.Time
	if orderData.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, orderData.ExpiresAt); err == nil {
			tUTC := t.UTC()
			expiresAtTime = &tUTC
		}
	}
	if expiresAtTime == nil {
		t := time.Now().UTC().Add(time.Duration(expiryMin) * time.Minute)
		expiresAtTime = &t
	}

	return &payment.PaymentResponse{
		GatewayOrderID:     orderData.OrderID,
		GatewayID:          a.ID(),
		Method:             payment.MethodBinancePay,
		Channel:            "USDT",
		OriginalAmount:     req.Amount,
		UniqueCode:         orderData.UniqueCode,
		TotalAmount:        req.Amount,
		CryptoAmountStr:    orderData.TotalAmountStr,
		CryptoCurrency:     orderData.Currency,
		BinancePayID:       orderData.BinancePayID,
		BinanceAccountName: orderData.BinanceAccountName,
		BinanceUniQRURL:    orderData.BinanceUniQRURL,
		QRURL:              orderData.QRURL,
		ExpiresAt:          expiresAtTime,
		ExpiresInSeconds:   orderData.ExpiresInSeconds,
		RawData: map[string]interface{}{
			"order_id":         orderData.OrderID,
			"status":           orderData.Status,
			"total_amount_str": orderData.TotalAmountStr,
			"currency":         orderData.Currency,
			"binance_pay_id":   orderData.BinancePayID,
		},
	}, nil
}

// CheckStatus queries status of an order on PaymentG Binance
func (a *Adapter) CheckStatus(ctx context.Context, gatewayOrderID string) (*payment.PaymentStatusResponse, error) {
	if gatewayOrderID == "" {
		return nil, fmt.Errorf("gateway order ID tidak boleh kosong")
	}

	orderData, err := a.client.CheckOrderStatus(ctx, gatewayOrderID)
	if err != nil {
		return nil, err
	}

	var paidAtTime *time.Time
	if orderData.PaidAt != "" {
		if t, err := time.Parse(time.RFC3339, orderData.PaidAt); err == nil {
			tUTC := t.UTC()
			paidAtTime = &tUTC
		}
	}

	status := strings.ToUpper(strings.TrimSpace(orderData.Status))

	return &payment.PaymentStatusResponse{
		GatewayOrderID: orderData.OrderID,
		GatewayID:      a.ID(),
		Status:         status,
		PaidAt:         paidAtTime,
		TransactionID:  orderData.BinanceTxID,
		RawStatus:      orderData.Status,
	}, nil
}

// TriggerInstantCheck triggers immediate mutation check on PaymentG Binance ("Saya Sudah Bayar")
func (a *Adapter) TriggerInstantCheck(ctx context.Context, gatewayOrderID string) error {
	if gatewayOrderID == "" {
		return fmt.Errorf("gateway order ID tidak boleh kosong")
	}
	_, err := a.client.TriggerInstantMutationCheck(ctx, gatewayOrderID)
	return err
}

// WebhookPayload represents the structure sent by PaymentG Binance webhook
type WebhookPayload struct {
	Event          string  `json:"event"`
	IsSandbox      bool    `json:"is_sandbox"`
	OrderID        string  `json:"order_id"`
	ReferenceID    string  `json:"reference_id"`
	Currency       string  `json:"currency"`
	OriginalAmount float64 `json:"original_amount"`
	UniqueCode     int     `json:"unique_code"`
	TotalAmount    float64 `json:"total_amount"`
	TotalAmountStr string  `json:"total_amount_str"`
	PaidAt         string  `json:"paid_at"`
	BinanceTxID    string  `json:"binance_tx_id"`
	Timestamp      int64   `json:"timestamp"`
}

// VerifyWebhook validates HMAC signature and anti-replay timestamp from PaymentG Binance
func (a *Adapter) VerifyWebhook(ctx context.Context, r *http.Request) (*payment.WebhookNotification, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	apiKey := a.client.getAPIKey()

	// 1. Signature Verification: X-Webhook-Signature
	receivedSig := r.Header.Get("X-Webhook-Signature")
	if apiKey != "" && receivedSig != "" {
		mac := hmac.New(sha256.New, []byte(apiKey))
		mac.Write(bodyBytes)
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(receivedSig), []byte(expectedSig)) {
			log.Printf("[SECURITY ALERT] Invalid webhook signature from Binance Pay (received: %s, expected: %s)", receivedSig, expectedSig)
			return nil, fmt.Errorf("invalid webhook signature")
		}
	}

	var payload WebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse Binance webhook JSON: %w", err)
	}

	if payload.ReferenceID == "" {
		return nil, fmt.Errorf("webhook payload missing reference_id")
	}

	// 2. Anti-Replay: timestamp check (max 5 minutes = 300 seconds)
	if payload.Timestamp > 0 {
		nowSec := time.Now().Unix()
		diff := math.Abs(float64(nowSec - payload.Timestamp))
		if diff > 300 {
			log.Printf("[SECURITY ALERT] Webhook expired / replay attack detected (diff: %.0fs)", diff)
			return nil, fmt.Errorf("webhook expired or replay attack detected")
		}
	}

	var paidAtTime *time.Time
	if payload.PaidAt != "" {
		if t, err := time.Parse(time.RFC3339, payload.PaidAt); err == nil {
			tUTC := t.UTC()
			paidAtTime = &tUTC
		}
	}

	isVerified := true
	// Optional: Double-check with server if API key is configured
	if payload.OrderID != "" && apiKey != "" {
		checkData, err := a.client.CheckOrderStatus(ctx, payload.OrderID)
		if err == nil && checkData != nil && checkData.Status != "PAID" {
			log.Printf("[SECURITY ALERT] Binance Pay double-check reported status '%s' instead of PAID for %s", checkData.Status, payload.ReferenceID)
			isVerified = false
		}
	}

	return &payment.WebhookNotification{
		Event:           payload.Event,
		GatewayID:       a.ID(),
		InvoiceNumber:   payload.ReferenceID,
		GatewayOrderID:  payload.OrderID,
		TransactionID:   payload.BinanceTxID,
		CryptoAmountStr: payload.TotalAmountStr,
		PaidAt:          paidAtTime,
		IsVerified:      isVerified,
		RawPayload:      bodyBytes,
	}, nil
}
