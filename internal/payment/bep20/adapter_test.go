package bep20

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

func TestBEP20Adapter_CreatePayment(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/orders" {
			t.Errorf("expected path /api/orders, got %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test_bep20_api_key" {
			t.Errorf("expected X-API-Key header 'test_bep20_api_key', got %s", r.Header.Get("X-API-Key"))
		}

		resp := APIResponse{
			Success: true,
			Data: &OrderData{
				OrderID:          "ord_bep20_12345",
				ReferenceID:      "INV-2026-TEST",
				Network:          "BSC (BEP-20)",
				ChainID:          56,
				Currency:         "USDT",
				TokenContract:    "0x55d398326f99059fF775485246999027B3197955",
				WalletAddress:    "0x71C67Ed3484A222ab20E083d9204E646A803D0A1",
				OriginalAmount:   10.5,
				UniqueCode:       14,
				TotalAmount:      10.514,
				TotalAmountStr:   "10.514",
				CexWarning:       "Pastikan Jumlah Bersih Diterima adalah TEPAT 10.514 USDT",
				Status:           "PENDING",
				QRURL:            "/api/orders/ord_bep20_12345/qr.png",
				ExpiresAt:        time.Now().Add(15 * time.Minute).Format(time.RFC3339),
				ExpiresInSeconds: 900,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		PaymentGBEP20APIURL: mockServer.URL,
		PaymentGBEP20APIKey: "test_bep20_api_key",
		USDToIDRRate:        18000.0,
	}

	adapter := NewAdapter(cfg)

	req := &payment.CreatePaymentRequest{
		InvoiceNumber: "INV-2026-TEST",
		Amount:        189000,
		AmountCrypto:  10.50,
		Method:        payment.MethodCrypto,
		Channel:       "BEP20",
	}

	resp, err := adapter.CreatePayment(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error creating payment: %v", err)
	}

	if resp.GatewayOrderID != "ord_bep20_12345" {
		t.Errorf("expected GatewayOrderID 'ord_bep20_12345', got '%s'", resp.GatewayOrderID)
	}
	if resp.WalletAddress != "0x71C67Ed3484A222ab20E083d9204E646A803D0A1" {
		t.Errorf("unexpected WalletAddress: %s", resp.WalletAddress)
	}
	if resp.CryptoAmountStr != "10.514" {
		t.Errorf("expected CryptoAmountStr '10.514', got '%s'", resp.CryptoAmountStr)
	}
	if resp.TokenContract != "0x55d398326f99059fF775485246999027B3197955" {
		t.Errorf("unexpected TokenContract: %s", resp.TokenContract)
	}
	if resp.Network != "BSC (BEP-20)" {
		t.Errorf("unexpected Network: %s", resp.Network)
	}
}

func TestBEP20Adapter_CheckStatus(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/orders/ord_bep20_12345/check" {
			t.Errorf("expected check path, got %s", r.URL.Path)
		}

		resp := CheckAPIResponse{
			Success: true,
			Data: &CheckStatusData{
				OrderID:        "ord_bep20_12345",
				Status:         "PAID",
				TotalAmountStr: "10.514",
				TxHash:         "0x8a9f7c6d5e4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f",
				PaidAt:         time.Now().Format(time.RFC3339),
				ScannedNow:     true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		PaymentGBEP20APIURL: mockServer.URL,
		PaymentGBEP20APIKey: "test_bep20_api_key",
	}

	adapter := NewAdapter(cfg)
	res, err := adapter.CheckStatus(context.Background(), "ord_bep20_12345")
	if err != nil {
		t.Fatalf("unexpected error checking status: %v", err)
	}

	if res.Status != "PAID" {
		t.Errorf("expected status PAID, got %s", res.Status)
	}
	if res.TransactionID != "0x8a9f7c6d5e4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f" {
		t.Errorf("unexpected transaction ID: %s", res.TransactionID)
	}
}

func TestBEP20Adapter_VerifyWebhook(t *testing.T) {
	apiKey := "test_secret_api_key"
	cfg := &config.Config{
		PaymentGBEP20APIURL: "http://localhost:3400",
		PaymentGBEP20APIKey: apiKey,
	}

	adapter := NewAdapter(cfg)

	payload := WebhookPayload{
		Event:          "payment.success",
		IsSandbox:      false,
		OrderID:        "ord_bep20_999",
		ReferenceID:    "INV-2026-999",
		Network:        "BSC (BEP-20)",
		Currency:       "USDT",
		TotalAmountStr: "10.514",
		TxHash:         "0xabcdef1234567890",
		Timestamp:      time.Now().Unix(),
	}

	bodyBytes, _ := json.Marshal(payload)

	// 1. Valid Signature Test
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write(bodyBytes)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook/usdt_bep20", bytes.NewBuffer(bodyBytes))
	req.Header.Set("X-Webhook-Signature", sig)

	notif, err := adapter.VerifyWebhook(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on valid webhook: %v", err)
	}
	if notif.InvoiceNumber != "INV-2026-999" {
		t.Errorf("expected invoice INV-2026-999, got %s", notif.InvoiceNumber)
	}
	if notif.TransactionID != "0xabcdef1234567890" {
		t.Errorf("expected tx 0xabcdef1234567890, got %s", notif.TransactionID)
	}

	// 2. Invalid Signature Test
	reqBadSig := httptest.NewRequest(http.MethodPost, "/webhook/usdt_bep20", bytes.NewBuffer(bodyBytes))
	reqBadSig.Header.Set("X-Webhook-Signature", "wrong_signature_hex")

	_, errBad := adapter.VerifyWebhook(context.Background(), reqBadSig)
	if errBad == nil {
		t.Fatal("expected error for bad signature, got nil")
	}

	// 3. Expired Replay Attack Test
	expiredPayload := payload
	expiredPayload.Timestamp = time.Now().Unix() - 400 // 400s ago (> 300s)
	expiredBody, _ := json.Marshal(expiredPayload)

	macExp := hmac.New(sha256.New, []byte(apiKey))
	macExp.Write(expiredBody)
	sigExp := hex.EncodeToString(macExp.Sum(nil))

	reqExp := httptest.NewRequest(http.MethodPost, "/webhook/usdt_bep20", bytes.NewBuffer(expiredBody))
	reqExp.Header.Set("X-Webhook-Signature", sigExp)

	_, errExp := adapter.VerifyWebhook(context.Background(), reqExp)
	if errExp == nil {
		t.Fatal("expected error for expired timestamp replay attack, got nil")
	}
}
