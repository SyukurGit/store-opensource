package handler

import (
	"testing"

	"storeku/internal/models"
)

func TestApplyLanguageToProduct(t *testing.T) {
	prod := &models.Product{
		Name:          "Akun Netflix Premium 1 Bulan",
		Type:          "Akun Berbagi",
		Region:        "Indonesia",
		Warranty:      "Garansi 30 Hari",
		Description:   "Akun legal dan bergaransi penuh.",
		Terms:         "Dilarang mengganti email/password.",
		NameEN:        "Netflix Premium Account 1 Month",
		TypeEN:        "Shared Account",
		RegionEN:      "Indonesia (Global Access)",
		WarrantyEN:    "30-Day Full Replacement Warranty",
		DescriptionEN: "Legal premium account with full warranty.",
		TermsEN:       "Do not modify email or password.",
	}

	// 1. Test lang=en with complete translations
	prodEN := *prod
	applyLanguageToProduct(&prodEN, "en")

	if prodEN.Name != "Netflix Premium Account 1 Month" {
		t.Errorf("expected Name to be English, got '%s'", prodEN.Name)
	}
	if prodEN.Type != "Shared Account" {
		t.Errorf("expected Type to be English, got '%s'", prodEN.Type)
	}
	if prodEN.Region != "Indonesia (Global Access)" {
		t.Errorf("expected Region to be English, got '%s'", prodEN.Region)
	}
	if prodEN.Warranty != "30-Day Full Replacement Warranty" {
		t.Errorf("expected Warranty to be English, got '%s'", prodEN.Warranty)
	}
	if prodEN.Description != "Legal premium account with full warranty." {
		t.Errorf("expected Description to be English, got '%s'", prodEN.Description)
	}
	if prodEN.Terms != "Do not modify email or password." {
		t.Errorf("expected Terms to be English, got '%s'", prodEN.Terms)
	}

	// 2. Test fallback when English fields are empty
	prodFallback := &models.Product{
		Name:        "Canva Pro 1 Tahun",
		Type:        "Invite Link",
		Region:      "Global",
		Warranty:    "Garansi 1 Tahun",
		Description: "Aktivasi melalui link email.",
		Terms:       "Bisa akun lama atau baru.",
		// EN fields left empty
	}
	applyLanguageToProduct(prodFallback, "en")

	if prodFallback.Name != "Canva Pro 1 Tahun" {
		t.Errorf("expected fallback Name to remain Indonesian, got '%s'", prodFallback.Name)
	}
	if prodFallback.Type != "Invite Link" {
		t.Errorf("expected fallback Type to remain Indonesian, got '%s'", prodFallback.Type)
	}
	if prodFallback.Region != "Global" {
		t.Errorf("expected fallback Region to remain Indonesian, got '%s'", prodFallback.Region)
	}
	if prodFallback.Warranty != "Garansi 1 Tahun" {
		t.Errorf("expected fallback Warranty to remain Indonesian, got '%s'", prodFallback.Warranty)
	}
	if prodFallback.Description != "Aktivasi melalui link email." {
		t.Errorf("expected fallback Description to remain Indonesian, got '%s'", prodFallback.Description)
	}

	// 3. Test lang=id leaves Indonesian intact
	prodID := *prod
	applyLanguageToProduct(&prodID, "id")
	if prodID.Name != "Akun Netflix Premium 1 Bulan" {
		t.Errorf("expected Name to stay Indonesian under lang=id, got '%s'", prodID.Name)
	}
}
