package paymentg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"storeku/internal/config"
	"storeku/internal/payment"
)

// Adapter adapts PaymentG client to the unified payment.Gateway interface
type Adapter struct {
	cfg    *config.Config
	client *Client
}

// NewAdapter creates a new PaymentG adapter
func NewAdapter(cfg *config.Config) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: NewClient(cfg),
	}
}

// NewAdapterWithClient allows injecting custom client (e.g. for testing)
func NewAdapterWithClient(cfg *config.Config, client *Client) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: client,
	}
}

// ID returns the unique gateway identifier
func (a *Adapter) ID() string {
	return "PAYMENTG"
}

// Name returns the human-readable display name
func (a *Adapter) Name() string {
	return "PaymentG ShopeePay QRIS"
}

// SupportedMethods returns supported payment methods
func (a *Adapter) SupportedMethods() []payment.Method {
	return []payment.Method{payment.MethodQRIS}
}

// CreatePayment creates a dynamic QRIS order via PaymentG
func (a *Adapter) CreatePayment(ctx context.Context, req *payment.CreatePaymentRequest) (*payment.PaymentResponse, error) {
	if req.Amount <= 0 {
		return nil, fmt.Errorf("nominal pembayaran harus lebih dari 0")
	}

	metadata := req.Metadata
	if metadata == "" {
		metadata = fmt.Sprintf("Digimest Order %s", req.InvoiceNumber)
	}

	orderData, err := a.client.CreateQRISOrder(ctx, req.InvoiceNumber, req.Amount, metadata)
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
		t := time.Now().UTC().Add(15 * time.Minute)
		expiresAtTime = &t
	}

	return &payment.PaymentResponse{
		GatewayOrderID:   orderData.OrderID,
		GatewayID:        a.ID(),
		Method:           payment.MethodQRIS,
		Channel:          "SHOPEEPAY",
		OriginalAmount:   orderData.OriginalAmount,
		UniqueCode:       orderData.UniqueCode,
		TotalAmount:      orderData.TotalAmount,
		QRURL:            orderData.QRURL,
		ExpiresAt:        expiresAtTime,
		ExpiresInSeconds: orderData.ExpiresInSeconds,
		RawData: map[string]interface{}{
			"order_id":    orderData.OrderID,
			"status":      orderData.Status,
			"unique_code": orderData.UniqueCode,
		},
	}, nil
}

// CheckStatus queries status of an order on PaymentG
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
		TransactionID:  orderData.ShopeeTxID,
		TotalAmount:    orderData.TotalAmount,
		RawStatus:      orderData.Status,
	}, nil
}

// TriggerInstantCheck triggers immediate mutation check on PaymentG
func (a *Adapter) TriggerInstantCheck(ctx context.Context, gatewayOrderID string) error {
	if gatewayOrderID == "" {
		return fmt.Errorf("gateway order ID tidak boleh kosong")
	}
	return a.client.TriggerInstantMutationCheck(ctx, gatewayOrderID)
}

// WebhookPayload represents PaymentG webhook format
type WebhookPayload struct {
	Event          string `json:"event"`
	OrderID        string `json:"order_id"`
	ReferenceID    string `json:"reference_id"`
	OriginalAmount int64  `json:"original_amount"`
	UniqueCode     int    `json:"unique_code"`
	TotalAmount    int64  `json:"total_amount"`
	PaidAt         string `json:"paid_at"`
	ShopeeTxID     string `json:"shopee_tx_id"`
}

// VerifyWebhook validates and double-checks an incoming webhook callback from PaymentG
func (a *Adapter) VerifyWebhook(ctx context.Context, r *http.Request) (*payment.WebhookNotification, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook body: %w", err)
	}
	// Restore body for any subsequent readers
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var payload WebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PaymentG webhook payload: %w", err)
	}

	if payload.ReferenceID == "" {
		return nil, fmt.Errorf("webhook payload missing reference_id")
	}

	var paidAtTime *time.Time
	if payload.PaidAt != "" {
		if t, err := time.Parse(time.RFC3339, payload.PaidAt); err == nil {
			tUTC := t.UTC()
			paidAtTime = &tUTC
		}
	}

	// Server-side double-check verification: Query PaymentG API to verify payment validity
	isVerified := true
	if payload.OrderID != "" && a.cfg.PaymentGAPIKey != "" {
		pgOrder, err := a.client.CheckOrderStatus(ctx, payload.OrderID)
		if err != nil || pgOrder == nil || pgOrder.Status != "PAID" {
			log.Printf("[SECURITY ALERT] PaymentG webhook double-check failed for invoice %s (pgOrder: %v, err: %v)",
				payload.ReferenceID, pgOrder, err)
			isVerified = false
		}
	}

	return &payment.WebhookNotification{
		Event:          payload.Event,
		GatewayID:      a.ID(),
		InvoiceNumber:  payload.ReferenceID,
		GatewayOrderID: payload.OrderID,
		TransactionID:  payload.ShopeeTxID,
		TotalAmount:    payload.TotalAmount,
		PaidAt:         paidAtTime,
		IsVerified:     isVerified,
		RawPayload:     bodyBytes,
	}, nil
}
