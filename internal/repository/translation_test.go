package repository

import (
	"path/filepath"
	"testing"

	"storeku/internal/database"
	"storeku/internal/models"
)

func TestLocalProductTranslation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_trans.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := NewLocalProductRepository(db)

	req := &models.LocalProductRequest{
		Name:          "Akun Netflix 1 Bulan",
		Slug:          "netflix-1-bulan",
		Type:          "Akun Berbagi",
		Region:        "Indonesia",
		Warranty:      "Garansi 30 Hari",
		Description:   "Akun premium legal dan bergaransi.",
		Terms:         "Dilarang ganti password.",
		PriceIDR:      45000,
		PromoPriceIDR: 35000,
		StockKeys:     []string{"KEY1", "KEY2"},
		IsActive:      true,
	}

	prod, err := repo.Create(req)
	if err != nil {
		t.Fatalf("failed to create local product: %v", err)
	}

	// Update translation
	transReq := &models.UpdateTranslationRequest{
		NameEN:        "Netflix Account 1 Month",
		TypeEN:        "Shared Account",
		RegionEN:      "Indonesia (Global Access)",
		WarrantyEN:    "30 Days Replacement Warranty",
		DescriptionEN: "Legal premium account with full warranty.",
		TermsEN:       "Do not change account password.",
	}

	if err := repo.UpdateTranslation(prod.Slug, transReq); err != nil {
		t.Fatalf("failed to update translation: %v", err)
	}

	// Verify
	fetched, err := repo.GetByID(prod.ID)
	if err != nil {
		t.Fatalf("failed to get product by id: %v", err)
	}
	if fetched.NameEN != transReq.NameEN {
		t.Errorf("expected NameEN '%s', got '%s'", transReq.NameEN, fetched.NameEN)
	}
	if fetched.TypeEN != transReq.TypeEN {
		t.Errorf("expected TypeEN '%s', got '%s'", transReq.TypeEN, fetched.TypeEN)
	}
	if fetched.RegionEN != transReq.RegionEN {
		t.Errorf("expected RegionEN '%s', got '%s'", transReq.RegionEN, fetched.RegionEN)
	}
	if fetched.WarrantyEN != transReq.WarrantyEN {
		t.Errorf("expected WarrantyEN '%s', got '%s'", transReq.WarrantyEN, fetched.WarrantyEN)
	}
	if fetched.DescriptionEN != transReq.DescriptionEN {
		t.Errorf("expected DescriptionEN '%s', got '%s'", transReq.DescriptionEN, fetched.DescriptionEN)
	}
	if fetched.TermsEN != transReq.TermsEN {
		t.Errorf("expected TermsEN '%s', got '%s'", transReq.TermsEN, fetched.TermsEN)
	}
}

func TestProductOverrideTranslation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ov_trans.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := NewProductOverrideRepository(db)

	transReq := &models.UpdateTranslationRequest{
		NameEN:        "Canva Pro 1 Year Subscription",
		TypeEN:        "Invite Link",
		RegionEN:      "Global",
		WarrantyEN:    "1 Year Warranty",
		DescriptionEN: "Unlock all premium features of Canva.",
		TermsEN:       "Personal use only.",
	}

	slug := "canva-pro-1-year"
	if err := repo.UpdateTranslation(slug, transReq); err != nil {
		t.Fatalf("failed to update translation on override: %v", err)
	}

	// Verify
	fetched, err := repo.GetBySlug(slug)
	if err != nil {
		t.Fatalf("failed to get override by slug: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected override record, got nil")
	}
	if fetched.CustomNameEN != transReq.NameEN {
		t.Errorf("expected CustomNameEN '%s', got '%s'", transReq.NameEN, fetched.CustomNameEN)
	}
	if fetched.CustomTypeEN != transReq.TypeEN {
		t.Errorf("expected CustomTypeEN '%s', got '%s'", transReq.TypeEN, fetched.CustomTypeEN)
	}
	if fetched.CustomWarrantyEN != transReq.WarrantyEN {
		t.Errorf("expected CustomWarrantyEN '%s', got '%s'", transReq.WarrantyEN, fetched.CustomWarrantyEN)
	}
}
