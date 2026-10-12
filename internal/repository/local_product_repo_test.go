package repository

import (
	"path/filepath"
	"testing"

	"storeku/internal/database"
	"storeku/internal/models"
)

func TestLocalProductRepository(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_local_prod.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := NewLocalProductRepository(db)

	// 1. Test Create with 3 keys
	req := &models.LocalProductRequest{
		Name:              "Canva Pro 1 Tahun",
		Slug:              "canva_pro_1_tahun",
		Type:              "Design Tools",
		PriceIDR:          25000,
		ReferencePriceIDR: 50000,
		PromoPriceIDR:     25000,
		HasPromo:          true,
		StockKeys:         []string{"KEY-CANVA-01", "KEY-CANVA-02", "KEY-CANVA-03"},
		IsActive:          true,
	}

	prod, err := repo.Create(req)
	if err != nil {
		t.Fatalf("failed to create local product: %v", err)
	}

	if prod.ID == 0 || prod.Slug != "canva_pro_1_tahun" {
		t.Errorf("unexpected product data: %+v", prod)
	}
	if prod.Stock != 3 {
		t.Errorf("expected stock 3, got %d", prod.Stock)
	}
	if len(prod.StockKeys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(prod.StockKeys))
	}

	// 2. Test ClaimKeys
	claimed, err := repo.ClaimKeys(prod.Slug, 2)
	if err != nil {
		t.Fatalf("failed to claim keys: %v", err)
	}
	if len(claimed) != 2 || claimed[0] != "KEY-CANVA-01" || claimed[1] != "KEY-CANVA-02" {
		t.Errorf("unexpected claimed keys: %+v", claimed)
	}

	// Verify remaining stock
	updated, err := repo.GetByID(prod.ID)
	if err != nil {
		t.Fatalf("failed to get updated product: %v", err)
	}
	if updated.Stock != 1 || len(updated.StockKeys) != 1 || updated.StockKeys[0] != "KEY-CANVA-03" {
		t.Errorf("expected 1 remaining key, got: %+v", updated.StockKeys)
	}

	// 3. Test Claiming when 1 remaining key is taken
	claimedLast, err := repo.ClaimKeys(prod.Slug, 1)
	if err != nil {
		t.Fatalf("failed to claim last key: %v", err)
	}
	if len(claimedLast) != 1 || claimedLast[0] != "KEY-CANVA-03" {
		t.Errorf("unexpected last claimed key: %+v", claimedLast)
	}

	// 4. Test Empty Key Fallback (when 0 keys available)
	claimedEmpty, err := repo.ClaimKeys(prod.Slug, 1)
	if err != nil {
		t.Fatalf("claim empty keys should not error: %v", err)
	}
	if len(claimedEmpty) != 0 {
		t.Errorf("expected empty slice, got %+v", claimedEmpty)
	}

	// 5. Test Key Capping at Max 10
	lotsOfKeys := []string{
		"K1", "K2", "K3", "K4", "K5", "K6", "K7", "K8", "K9", "K10", "K11", "K12",
	}
	req2 := &models.LocalProductRequest{
		Name:      "ChatGPT Plus 1 Bulan",
		StockKeys: lotsOfKeys,
		IsActive:  true,
		PriceIDR:  45000,
	}
	prod2, err := repo.Create(req2)
	if err != nil {
		t.Fatalf("failed to create second product: %v", err)
	}
	if len(prod2.StockKeys) != 10 {
		t.Errorf("expected exactly 10 capped keys, got %d", len(prod2.StockKeys))
	}

	// 6. Test ToProduct conversion
	converted := prod2.ToProduct()
	if converted.Source != "LOCAL" || converted.Name != "ChatGPT Plus 1 Bulan" || converted.PriceIDR != 45000 {
		t.Errorf("unexpected converted product: %+v", converted)
	}

	// 7. Test GetAll & GetAllActive
	all, err := repo.GetAll()
	if err != nil || len(all) != 2 {
		t.Errorf("expected 2 products in GetAll, got %d", len(all))
	}

	// 8. Test Delete
	if err := repo.Delete(prod.ID); err != nil {
		t.Fatalf("failed to delete product: %v", err)
	}
	allAfter, _ := repo.GetAll()
	if len(allAfter) != 1 {
		t.Errorf("expected 1 product remaining after delete, got %d", len(allAfter))
	}
}
