package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/payment"
	"storeku/internal/repository"
	"storeku/internal/service"
)

type testMockGateway struct {
	id         string
	isVerified bool
}

func (m *testMockGateway) ID() string                              { return m.id }
func (m *testMockGateway) Name() string                            { return "Test Gateway " + m.id }
func (m *testMockGateway) SupportedMethods() []payment.Method      { return []payment.Method{payment.MethodQRIS} }
func (m *testMockGateway) CreatePayment(ctx context.Context, req *payment.CreatePaymentRequest) (*payment.PaymentResponse, error) {
	return nil, nil
}
func (m *testMockGateway) CheckStatus(ctx context.Context, gatewayOrderID string) (*payment.PaymentStatusResponse, error) {
	return nil, nil
}
func (m *testMockGateway) TriggerInstantCheck(ctx context.Context, gatewayOrderID string) error {
	return nil
}
func (m *testMockGateway) VerifyWebhook(ctx context.Context, r *http.Request) (*payment.WebhookNotification, error) {
	var payload struct {
		Event       string `json:"event"`
		ReferenceID string `json:"reference_id"`
		OrderID     string `json:"order_id"`
		ShopeeTxID  string `json:"shopee_tx_id"`
		TotalAmount int64  `json:"total_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("bad json: %w", err)
	}

	return &payment.WebhookNotification{
		Event:          payload.Event,
		GatewayID:      m.id,
		InvoiceNumber:  payload.ReferenceID,
		GatewayOrderID: payload.OrderID,
		TransactionID:  payload.ShopeeTxID,
		TotalAmount:    payload.TotalAmount,
		IsVerified:     m.isVerified,
	}, nil
}

func setupTestWebhookEnv(t *testing.T) (*WebhookHandler, *repository.OrderRepository, *service.OrderService) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orderRepo := repository.NewOrderRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	overrideRepo := repository.NewProductOverrideRepository(db)
	cfg := &config.Config{ProfitMarginPercent: 25.0, USDToIDRRate: 18000.0}
	pricing := service.NewPricingService(cfg, settingRepo)
	dcClient := service.NewDigitalCoreClient(cfg, pricing, overrideRepo)
	pgClient := service.NewPaymentGClient(cfg)
	resend := service.NewResendService(cfg)
	orderService := service.NewOrderService(orderRepo, pricing, dcClient, pgClient, resend)

	wh := NewWebhookHandler(orderService, pgClient, orderRepo)
	return wh, orderRepo, orderService
}

func TestHandlePaymentGWebhook_Verified(t *testing.T) {
	wh, repo, _ := setupTestWebhookEnv(t)

	registry := payment.NewRegistry()
	mockGW := &testMockGateway{id: "PAYMENTG", isVerified: true}
	registry.Register(mockGW)
	wh.SetPaymentRegistry(registry)

	exp := time.Now().Add(15 * time.Minute)
	order := &models.Order{
		InvoiceNumber:     "INV-WH-001",
		ClaimCode:         "CLM-WH-001",
		ProductSlug:       "test-slug",
		ProductName:       "Test Product",
		Quantity:          1,
		TotalAmount:       50000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	payload := map[string]interface{}{
		"event":        "payment.success",
		"reference_id": "INV-WH-001",
		"order_id":     "GW-ORD-001",
		"shopee_tx_id": "SHP-TX-001",
		"total_amount": 50000,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/webhook/paymentg", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	wh.HandlePaymentGWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	updated, err := repo.GetOrderByInvoice("INV-WH-001")
	if err != nil {
		t.Fatalf("failed to get updated order: %v", err)
	}
	if updated.PaymentStatus != "PAID" {
		t.Errorf("expected status PAID, got %s", updated.PaymentStatus)
	}
}

func TestHandlePaymentGWebhook_UnverifiedRejected(t *testing.T) {
	wh, repo, _ := setupTestWebhookEnv(t)

	registry := payment.NewRegistry()
	mockGW := &testMockGateway{id: "PAYMENTG", isVerified: false}
	registry.Register(mockGW)
	wh.SetPaymentRegistry(registry)

	exp := time.Now().Add(15 * time.Minute)
	order := &models.Order{
		InvoiceNumber:     "INV-WH-002",
		ClaimCode:         "CLM-WH-002",
		ProductSlug:       "test-slug",
		ProductName:       "Test Product",
		Quantity:          1,
		TotalAmount:       50000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	payload := map[string]interface{}{
		"event":        "payment.success",
		"reference_id": "INV-WH-002",
		"order_id":     "GW-ORD-002",
		"shopee_tx_id": "FAKE-TX",
		"total_amount": 50000,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/webhook/paymentg", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	wh.HandlePaymentGWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Order must REMAIN PENDING because webhook was unverified!
	updated, err := repo.GetOrderByInvoice("INV-WH-002")
	if err != nil {
		t.Fatalf("failed to get updated order: %v", err)
	}
	if updated.PaymentStatus != "PENDING" {
		t.Errorf("expected status PENDING after unverified webhook, got %s", updated.PaymentStatus)
	}
}

func TestHandleGatewayWebhook_DynamicRouting(t *testing.T) {
	wh, repo, _ := setupTestWebhookEnv(t)

	registry := payment.NewRegistry()
	mockMidtrans := &testMockGateway{id: "MIDTRANS", isVerified: true}
	registry.Register(mockMidtrans)
	wh.SetPaymentRegistry(registry)

	exp := time.Now().Add(15 * time.Minute)
	order := &models.Order{
		InvoiceNumber:     "INV-WH-003",
		ClaimCode:         "CLM-WH-003",
		ProductSlug:       "test-slug",
		ProductName:       "Test Product",
		Quantity:          1,
		TotalAmount:       100000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/api/webhook/{gateway}", wh.HandleGatewayWebhook)

	payload := map[string]interface{}{
		"event":        "payment.success",
		"reference_id": "INV-WH-003",
		"order_id":     "MTR-001",
		"shopee_tx_id": "MTR-TX-100",
		"total_amount": 100000,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/webhook/midtrans", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", w.Code, w.Body.String())
	}

	updated, err := repo.GetOrderByInvoice("INV-WH-003")
	if err != nil {
		t.Fatalf("failed to get updated order: %v", err)
	}
	if updated.PaymentStatus != "PAID" {
		t.Errorf("expected status PAID, got %s", updated.PaymentStatus)
	}
}
