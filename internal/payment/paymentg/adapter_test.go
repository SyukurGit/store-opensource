package paymentg

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"storeku/internal/config"
	"storeku/internal/payment"
)

func TestPaymentGAdapter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/orders" {
			// Create order mock
			resp := APIResponse{
				Success: true,
				Data: &OrderData{
					OrderID:          "PG-ORD-12345",
					ReferenceID:      "INV-2026-001",
					OriginalAmount:   50000,
					UniqueCode:       123,
					TotalAmount:      50123,
					Status:           "PENDING",
					QRURL:            "/qris/image.png",
					ExpiresAt:        time.Now().Add(15 * time.Minute).Format(time.RFC3339),
					ExpiresInSeconds: 900,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/orders/PG-ORD-12345" {
			// Check status mock
			resp := APIResponse{
				Success: true,
				Data: &OrderData{
					OrderID:     "PG-ORD-12345",
					ReferenceID: "INV-2026-001",
					Status:      "PAID",
					ShopeeTxID:  "SHP-TX-999",
					TotalAmount: 50123,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/orders/PG-ORD-12345/check" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := &config.Config{
		PaymentGAPIURL: ts.URL,
		PaymentGAPIKey: "test-api-key",
	}

	adapter := NewAdapter(cfg)

	// 1. Identity checks
	if adapter.ID() != "PAYMENTG" {
		t.Fatalf("expected ID PAYMENTG, got %s", adapter.ID())
	}
	if len(adapter.SupportedMethods()) == 0 || adapter.SupportedMethods()[0] != payment.MethodQRIS {
		t.Fatalf("expected SupportedMethods to include QRIS")
	}

	// 2. CreatePayment
	ctx := context.Background()
	payResp, err := adapter.CreatePayment(ctx, &payment.CreatePaymentRequest{
		InvoiceNumber: "INV-2026-001",
		Amount:        50000,
		ExpiryMinutes: 15,
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if payResp.GatewayOrderID != "PG-ORD-12345" {
		t.Fatalf("expected order ID PG-ORD-12345, got %s", payResp.GatewayOrderID)
	}
	if payResp.TotalAmount != 50123 {
		t.Fatalf("expected total 50123, got %d", payResp.TotalAmount)
	}

	// 3. TriggerInstantCheck
	if err := adapter.TriggerInstantCheck(ctx, "PG-ORD-12345"); err != nil {
		t.Fatalf("TriggerInstantCheck failed: %v", err)
	}

	// 4. CheckStatus
	statusResp, err := adapter.CheckStatus(ctx, "PG-ORD-12345")
	if err != nil {
		t.Fatalf("CheckStatus failed: %v", err)
	}
	if statusResp.Status != "PAID" {
		t.Fatalf("expected status PAID, got %s", statusResp.Status)
	}
	if statusResp.TransactionID != "SHP-TX-999" {
		t.Fatalf("expected transaction ID SHP-TX-999, got %s", statusResp.TransactionID)
	}

	// 5. VerifyWebhook
	webhookPayload := WebhookPayload{
		Event:          "payment.success",
		OrderID:        "PG-ORD-12345",
		ReferenceID:    "INV-2026-001",
		TotalAmount:    50123,
		ShopeeTxID:     "SHP-TX-999",
	}
	payloadBytes, _ := json.Marshal(webhookPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/webhook/paymentg", bytes.NewBuffer(payloadBytes))

	notif, err := adapter.VerifyWebhook(ctx, req)
	if err != nil {
		t.Fatalf("VerifyWebhook failed: %v", err)
	}
	if !notif.IsVerified {
		t.Fatalf("expected webhook to be verified")
	}
	if notif.InvoiceNumber != "INV-2026-001" {
		t.Fatalf("expected invoice INV-2026-001, got %s", notif.InvoiceNumber)
	}
}
