package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/repository"
	"storeku/internal/service"
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

func TestValidateVoucher_MultiProvider(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	voucherRepo := repository.NewVoucherRepository(db)
	localRepo := repository.NewLocalProductRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	cfg := &config.Config{
		VoucherSecretKey: "test_voucher_secret",
	}

	voucherService := service.NewVoucherService(cfg, voucherRepo)

	// Create test voucher
	expTime := time.Now().Add(24 * time.Hour)
	expStr := expTime.UTC().Format(time.RFC3339)
	sig := voucherService.GenerateSignature("HEMAT5K", 5000, 10000, 100, expStr)
	v := &models.Voucher{
		Code:        "HEMAT5K",
		DiscountIDR: 5000,
		MinSpendIDR: 10000,
		MaxUsage:    100,
		IsActive:    true,
		ExpiresAt:   &expTime,
		Signature:   sig,
	}
	if err := voucherRepo.Create(v); err != nil {
		t.Fatalf("failed to create voucher: %v", err)
	}

	// Create a non-DigitalCore product (e.g. Silvex or Local product)
	lpReq := &models.LocalProductRequest{
		Name:        "ChatGPT Plus Reseller (Silvex AI)",
		Type:        "Direct Account",
		PriceIDR:    50000,
		StockKeys:   []string{"key1", "key2"},
		IsActive:    true,
		Description: "Silvex AI product",
	}
	lp, err := localRepo.Create(lpReq)
	if err != nil {
		t.Fatalf("failed to create local product: %v", err)
	}

	h := NewPublicHandler(cfg, nil, nil, settingRepo)
	h.SetVoucherService(voucherService)
	h.SetLocalProductRepo(localRepo)

	// Test validate voucher on Silvex/local product
	reqBody := models.ValidateVoucherRequest{
		Code:        "HEMAT5K",
		ProductSlug: lp.Slug,
		Quantity:    1,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/vouchers/validate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ValidateVoucher(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d with body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool                         `json:"success"`
		Data    models.ValidateVoucherResponse `json:"data"`
		Error   string                       `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !resp.Success || !resp.Data.Valid {
		t.Fatalf("expected voucher to be valid, got resp: %+v", resp)
	}
	if resp.Data.DiscountIDR != 5000 {
		t.Errorf("expected discount 5000, got %d", resp.Data.DiscountIDR)
	}
	if resp.Data.SubtotalOriginal != 50000 {
		t.Errorf("expected subtotal original 50000, got %d", resp.Data.SubtotalOriginal)
	}
	if resp.Data.SubtotalFinal != 45000 {
		t.Errorf("expected subtotal final 45000, got %d", resp.Data.SubtotalFinal)
	}
}
