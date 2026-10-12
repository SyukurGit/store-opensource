package repository

import (
	"path/filepath"
	"testing"

	"storeku/internal/database"
	"storeku/internal/models"
)

func TestProductOverrideRepositoryCustomFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := NewProductOverrideRepository(db)

	override := &models.ProductOverride{
		ProductSlug:       "test_product_override",
		CustomName:        "Test Product Custom",
		CustomType:        "Activation Link",
		CustomRegion:      "Global",
		CustomWarranty:    "Garansi 30 Hari Replace",
		CustomStock:       42,
		CustomPriceIDR:    150000,
		PromoPriceIDR:     125000,
		CustomDescription: "Deskripsi kustom",
		CustomTerms:       "Syarat kustom",
		IsActive:          true,
		IsForceOutOfStock: false,
		BaselineModalIDR:  100000,
		IsAutoSuspended:   false,
	}

	if err := repo.Upsert(override); err != nil {
		t.Fatalf("failed to upsert override: %v", err)
	}

	// 1. Test GetBySlug
	fetched, err := repo.GetBySlug("test_product_override")
	if err != nil {
		t.Fatalf("failed to get by slug: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected override, got nil")
	}
	if fetched.CustomType != "Activation Link" {
		t.Errorf("expected CustomType 'Activation Link', got '%s'", fetched.CustomType)
	}
	if fetched.CustomRegion != "Global" {
		t.Errorf("expected CustomRegion 'Global', got '%s'", fetched.CustomRegion)
	}
	if fetched.CustomWarranty != "Garansi 30 Hari Replace" {
		t.Errorf("expected CustomWarranty 'Garansi 30 Hari Replace', got '%s'", fetched.CustomWarranty)
	}
	if fetched.CustomStock != 42 {
		t.Errorf("expected CustomStock 42, got %d", fetched.CustomStock)
	}

	// 2. Test GetAll
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("failed to get all overrides: %v", err)
	}
	if ov, ok := all["test_product_override"]; !ok || ov.CustomStock != 42 {
		t.Errorf("expected override in GetAll with CustomStock 42")
	}

	// 3. Test Delete
	if err := repo.Delete("test_product_override"); err != nil {
		t.Fatalf("failed to delete override: %v", err)
	}
	deleted, err := repo.GetBySlug("test_product_override")
	if err != nil || deleted != nil {
		t.Errorf("expected nil after delete, got %v", deleted)
	}
}
