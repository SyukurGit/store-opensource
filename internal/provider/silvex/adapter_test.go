package silvex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"storeku/internal/config"
	"storeku/internal/provider"
)

type mockPricingCalc struct{}

func (m *mockPricingCalc) ConvertUSDToIDR(usd float64) int64 {
	return int64(usd * 18000 * 1.25) // with margin
}
func (m *mockPricingCalc) GetUSDRate() float64 {
	return 18000
}

func TestNormalizeDeliveredItems(t *testing.T) {
	items := []DeliveredItem{
		{Title: "Email", Content: "user@example.com"},
		{Title: "Password", Content: "secret123"},
		{Title: "", Content: "Direct license key without title"},
	}

	normalized := NormalizeDeliveredItems(items)
	if len(normalized) != 3 {
		t.Fatalf("expected 3 items, got %d", len(normalized))
	}
	if normalized[0] != "Email: user@example.com" {
		t.Errorf("unexpected format: %s", normalized[0])
	}
	if normalized[2] != "Direct license key without title" {
		t.Errorf("unexpected format: %s", normalized[2])
	}
}

func TestNormalizeProduct(t *testing.T) {
	raw := RawProduct{
		ProductID:   "claude-pro-1m",
		Name:        "Claude Pro 1 Month",
		Price:       4.5,
		Stock:       15,
		Description: "Official Anthropic Claude Pro subscription",
	}

	calc := &mockPricingCalc{}
	p := NormalizeProduct(raw, calc)

	if p.Slug != "silvex_claude_pro_1_month" {
		t.Errorf("expected slug 'silvex_claude_pro_1_month', got '%s'", p.Slug)
	}
	if p.Source != "SILVEX" {
		t.Errorf("expected source 'SILVEX', got '%s'", p.Source)
	}
	if p.PriceUSD != 4.5 {
		t.Errorf("expected PriceUSD 4.5, got %f", p.PriceUSD)
	}
	if p.ModalIDR != 81000 {
		t.Errorf("expected ModalIDR 81000, got %d", p.ModalIDR)
	}
}

func TestSilvexAdapterFulfillInstantAndPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test_silvex_key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "unauthorized"})
			return
		}

		if r.URL.Path == "/order" && r.Method == http.MethodPost {
			var req OrderRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req.ProductID == "manual-item" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(OrderResponse{
					OK:      true,
					OrderID: "slvx-ord-manual",
					Status:  "Pending",
					Total:   10.0,
				})
				return
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(OrderResponse{
				OK:      true,
				OrderID: "slvx-ord-instant",
				Status:  "Completed",
				Total:   4.5,
				Balance: 20.5,
				Delivered: []DeliveredItem{
					{Title: "Account", Content: "user:pass"},
				},
			})
			return
		}

		if r.URL.Path == "/balance" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BalanceResponse{
				OK:      true,
				Balance: 25.5,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		SilvexBaseURL: server.URL,
		SilvexAPIKey:  "test_silvex_key",
	}

	adapter := NewAdapter(cfg, &mockPricingCalc{})
	ctx := context.Background()

	// 1. Test CheckHealth
	health, err := adapter.CheckHealth(ctx)
	if err != nil || !health.Success || health.Balance != 25.5 {
		t.Fatalf("CheckHealth failed: %v, health: %+v", err, health)
	}

	// 2. Test Instant Fulfill
	instRes, err := adapter.Fulfill(ctx, &provider.FulfillmentRequest{
		ProductSlug:    "silvex_claude_pro",
		ProviderItemID: "instant-item",
		Quantity:       1,
	})
	if err != nil || !instRes.Success || instRes.ProviderOrderID != "slvx-ord-instant" {
		t.Fatalf("Instant Fulfill failed: %v, res: %+v", err, instRes)
	}
	if len(instRes.DeliveredItems) != 1 || instRes.DeliveredItems[0] != "Account: user:pass" {
		t.Fatalf("Unexpected delivered items: %v", instRes.DeliveredItems)
	}

	// 3. Test Pending/Manual Fulfill
	pendRes, err := adapter.Fulfill(ctx, &provider.FulfillmentRequest{
		ProductSlug:    "silvex_manual",
		ProviderItemID: "manual-item",
		Quantity:       1,
	})
	if pendRes == nil || pendRes.Success || pendRes.ErrorMessage != "PENDING_MANUAL_DELIVERY" {
		t.Fatalf("Expected PENDING_MANUAL_DELIVERY, got: %+v, err: %v", pendRes, err)
	}
}
