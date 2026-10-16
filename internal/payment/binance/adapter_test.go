package binance

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"storeku/internal/config"
	"storeku/internal/payment"
)

func TestBinanceAdapter_CreatePaymentAndCheckStatus(t *testing.T) {
	testApiKey := "ak_test_binance_secret_12345"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != testApiKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success": false, "error": "unauthorized"}`))
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/orders":
			var req CreateOrderRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(APIResponse{
				Success: true,
				Data: &OrderData{
					OrderID:            "ord_binance_test_01",
					ReferenceID:        req.ReferenceID,
					Currency:           "USDT",
					OriginalAmount:     req.Amount,
					UniqueCode:         14,
					TotalAmount:        req.Amount + 0.014,
					TotalAmountStr:     "0.814",
					BinancePayID:       "857242044",
					BinanceAccountName: "AirpayX7",
					BinanceUniQRURL:    "https://app.binance.com/uni-qr/test",
					Status:             "PENDING",
					QRURL:              "/api/sandbox/orders/ord_binance_test_01/qr.png",
					ExpiresAt:          time.Now().Add(15 * time.Minute).Format(time.RFC3339),
					ExpiresInSeconds:   900,
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/orders/ord_binance_test_01":
			_ = json.NewEncoder(w).Encode(APIResponse{
				Success: true,
				Data: &OrderData{
					OrderID:        "ord_binance_test_01",
					Status:         "PAID",
					BinanceTxID:    "M_P_998877665544",
					PaidAt:         time.Now().Format(time.RFC3339),
					TotalAmountStr: "0.814",
				},
			})

		case r.Method == http.MethodPost && r.URL.Path == "/orders/ord_binance_test_01/check":
			_ = json.NewEncoder(w).Encode(APIResponse{
				Success: true,
				Data: &OrderData{
					OrderID:     "ord_binance_test_01",
					Status:      "PAID",
					BinanceTxID: "M_P_998877665544",
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		PaymentGBinanceAPIURL: ts.URL,
		PaymentGBinanceAPIKey: testApiKey,
		USDToIDRRate:          18000.0,
	}

	adapter := NewAdapter(cfg)

	// 1. Test CreatePayment
	resp, err := adapter.CreatePayment(context.Background(), &payment.CreatePaymentRequest{
		InvoiceNumber: "INV-BINANCE-001",
		Amount:        18000, // 18.000 IDR / 18.000 kurs = 1.00 USDT
		ExpiryMinutes: 15,
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if resp.GatewayOrderID != "ord_binance_test_01" {
		t.Errorf("expected GatewayOrderID 'ord_binance_test_01', got '%s'", resp.GatewayOrderID)
	}
	if resp.CryptoAmountStr != "0.814" {
		t.Errorf("expected CryptoAmountStr '0.814', got '%s'", resp.CryptoAmountStr)
	}
	if resp.BinancePayID != "857242044" {
		t.Errorf("expected BinancePayID '857242044', got '%s'", resp.BinancePayID)
	}

	// 2. Test CheckStatus
	statusResp, err := adapter.CheckStatus(context.Background(), "ord_binance_test_01")
	if err != nil {
		t.Fatalf("CheckStatus failed: %v", err)
	}
	if statusResp.Status != "PAID" {
		t.Errorf("expected status 'PAID', got '%s'", statusResp.Status)
	}
	if statusResp.TransactionID != "M_P_998877665544" {
		t.Errorf("expected TransactionID 'M_P_998877665544', got '%s'", statusResp.TransactionID)
	}

	// 3. Test TriggerInstantCheck
	if err := adapter.TriggerInstantCheck(context.Background(), "ord_binance_test_01"); err != nil {
		t.Fatalf("TriggerInstantCheck failed: %v", err)
	}
}

func TestBinanceAdapter_VerifyWebhook(t *testing.T) {
	testApiKey := "ak_test_secret_for_webhook"
	cfg := &config.Config{
		PaymentGBinanceAPIKey: testApiKey,
	}
	adapter := NewAdapter(cfg)

	// Case 1: Valid Webhook with correct signature
	payload := WebhookPayload{
		Event:          "payment.success",
		OrderID:        "ord_wh_001",
		ReferenceID:    "INV-WH-001",
		Currency:       "USDT",
		TotalAmountStr: "0.814",
		BinanceTxID:    "M_P_11223344",
		Timestamp:      time.Now().Unix(),
	}
	bodyBytes, _ := json.Marshal(payload)

	mac := hmac.New(sha256.New, []byte(testApiKey))
	mac.Write(bodyBytes)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook/binance", bytes.NewBuffer(bodyBytes))
	req.Header.Set("X-Webhook-Signature", sig)

	notif, err := adapter.VerifyWebhook(context.Background(), req)
	if err != nil {
		t.Fatalf("VerifyWebhook failed for valid signature: %v", err)
	}
	if !notif.IsVerified {
		t.Errorf("expected IsVerified to be true")
	}
	if notif.InvoiceNumber != "INV-WH-001" {
		t.Errorf("expected invoice 'INV-WH-001', got '%s'", notif.InvoiceNumber)
	}
	if notif.TransactionID != "M_P_11223344" {
		t.Errorf("expected txID 'M_P_11223344', got '%s'", notif.TransactionID)
	}
	if notif.CryptoAmountStr != "0.814" {
		t.Errorf("expected CryptoAmountStr '0.814', got '%s'", notif.CryptoAmountStr)
	}

	// Case 2: Bad signature must be rejected
	reqBadSig := httptest.NewRequest(http.MethodPost, "/webhook/binance", bytes.NewBuffer(bodyBytes))
	reqBadSig.Header.Set("X-Webhook-Signature", "wrong_signature_hex")
	_, errBad := adapter.VerifyWebhook(context.Background(), reqBadSig)
	if errBad == nil {
		t.Errorf("expected error for bad signature, got nil")
	}

	// Case 3: Expired timestamp (> 5 minutes ago) must be rejected
	expiredPayload := payload
	expiredPayload.Timestamp = time.Now().Unix() - 400 // 400s > 300s limit
	expBody, _ := json.Marshal(expiredPayload)

	macExp := hmac.New(sha256.New, []byte(testApiKey))
	macExp.Write(expBody)
	sigExp := hex.EncodeToString(macExp.Sum(nil))

	reqExp := httptest.NewRequest(http.MethodPost, "/webhook/binance", bytes.NewBuffer(expBody))
	reqExp.Header.Set("X-Webhook-Signature", sigExp)
	_, errExp := adapter.VerifyWebhook(context.Background(), reqExp)
	if errExp == nil {
		t.Errorf("expected error for expired timestamp, got nil")
	}
}
