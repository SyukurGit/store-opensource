package repository

import (
	"path/filepath"
	"testing"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
)

func TestVoucherRepository(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_voucher.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := NewVoucherRepository(db)

	// 1. Create Voucher
	v1 := &models.Voucher{
		Code:        "HEMAT10K",
		DiscountIDR: 10000,
		MinSpendIDR: 20000,
		MaxUsage:    2,
		UsedCount:   0,
		IsActive:    true,
		Signature:   "sig_valid_123",
		Note:        "Voucher promo test",
	}

	if err := repo.Create(v1); err != nil {
		t.Fatalf("failed to create voucher: %v", err)
	}
	if v1.ID <= 0 {
		t.Fatalf("expected positive voucher ID, got %d", v1.ID)
	}

	// 2. Duplicate code rejection
	vDup := &models.Voucher{
		Code:        "HEMAT10K",
		DiscountIDR: 5000,
		MaxUsage:    1,
		IsActive:    true,
		Signature:   "sig_dup",
	}
	if err := repo.Create(vDup); err == nil {
		t.Fatalf("expected error on duplicate voucher code, got nil")
	}

	// 3. GetByCode
	found, err := repo.GetByCode("hemat10k") // test case insensitivity
	if err != nil {
		t.Fatalf("failed to get voucher by code: %v", err)
	}
	if found.Code != "HEMAT10K" || found.DiscountIDR != 10000 || found.MaxUsage != 2 {
		t.Fatalf("mismatched voucher data: %+v", found)
	}

	// 4. ConsumeVoucherAtomic #1
	consumed1, err := repo.ConsumeVoucherAtomic("HEMAT10K")
	if err != nil {
		t.Fatalf("consume #1 failed: %v", err)
	}
	if consumed1.UsedCount != 1 {
		t.Fatalf("expected used_count 1, got %d", consumed1.UsedCount)
	}

	// 5. ConsumeVoucherAtomic #2 (last quota)
	consumed2, err := repo.ConsumeVoucherAtomic("HEMAT10K")
	if err != nil {
		t.Fatalf("consume #2 failed: %v", err)
	}
	if consumed2.UsedCount != 2 {
		t.Fatalf("expected used_count 2, got %d", consumed2.UsedCount)
	}

	// 6. ConsumeVoucherAtomic #3 (should fail - quota exhausted)
	_, errQuota := repo.ConsumeVoucherAtomic("HEMAT10K")
	if errQuota == nil {
		t.Fatalf("expected quota exhausted error, got nil")
	}

	// 7. Revert usage
	if err := repo.RevertVoucherUsage("HEMAT10K"); err != nil {
		t.Fatalf("failed to revert usage: %v", err)
	}
	reverted, _ := repo.GetByCode("HEMAT10K")
	if reverted.UsedCount != 1 {
		t.Fatalf("expected used_count 1 after revert, got %d", reverted.UsedCount)
	}

	// 8. Test Expired Voucher Rejection
	past := time.Now().Add(-1 * time.Hour)
	vExp := &models.Voucher{
		Code:        "EXPIRED10",
		DiscountIDR: 5000,
		MaxUsage:    10,
		ExpiresAt:   &past,
		IsActive:    true,
		Signature:   "sig_exp",
	}
	if err := repo.Create(vExp); err != nil {
		t.Fatalf("failed to create expired voucher: %v", err)
	}
	_, errExp := repo.ConsumeVoucherAtomic("EXPIRED10")
	if errExp == nil {
		t.Fatalf("expected expired error, got nil")
	}

	// 9. Toggle Active
	if err := repo.ToggleActive(v1.ID); err != nil {
		t.Fatalf("failed to toggle active: %v", err)
	}
	toggled, _ := repo.GetByID(v1.ID)
	if toggled.IsActive {
		t.Fatalf("expected is_active to be false after toggle")
	}

	// 10. Delete
	if err := repo.Delete(vExp.ID); err != nil {
		t.Fatalf("failed to delete voucher: %v", err)
	}
	list, err := repo.GetAll()
	if err != nil {
		t.Fatalf("failed to get all: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 voucher in list, got %d", len(list))
	}
}
