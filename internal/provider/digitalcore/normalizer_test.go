package digitalcore

import (
	"testing"
)

type mockPricing struct{}

func (m *mockPricing) ConvertUSDToIDR(usd float64) int64 {
	return int64(usd * 18000)
}
func (m *mockPricing) GetUSDRate() float64 {
	return 16500
}

func TestNormalizeRawProduct(t *testing.T) {
	raw := RawProduct{
		ID:          "123",
		Name:        "Gemini Advanced 1 Year",
		Type:        "Subscription",
		Logo:        "/static/gemini.png",
		Region:      "Global",
		Status:      "active",
		Stock:       50,
		StockCapped: false,
		Warranty:    "1 Year",
		Desc:        "Official Google Gemini subscription",
		PriceFrom:   5.0,
	}

	calc := &mockPricing{}
	prod := NormalizeRawProduct(raw, "https://digitalcore.top", calc)

	if prod.Slug != "gemini_advanced_1_year" {
		t.Errorf("expected slug 'gemini_advanced_1_year', got '%s'", prod.Slug)
	}
	if prod.Source != "DIGITALCORE" {
		t.Errorf("expected source 'DIGITALCORE', got '%s'", prod.Source)
	}
	if prod.PriceUSD != 5.0 {
		t.Errorf("expected PriceUSD 5.0, got %f", prod.PriceUSD)
	}
	if prod.PriceIDR != 90000 {
		t.Errorf("expected PriceIDR 90000, got %d", prod.PriceIDR)
	}
	if prod.ModalIDR != 82500 {
		t.Errorf("expected ModalIDR 82500, got %d", prod.ModalIDR)
	}
	if prod.Logo != "https://digitalcore.top/static/gemini.png" {
		t.Errorf("expected full Logo URL, got '%s'", prod.Logo)
	}
}
