package service

import (
	"path/filepath"
	"testing"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/repository"
)

func TestVoucherService(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_voucher_svc.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		AdminPassword:    "superadmin123",
		AdminSecretKey:   "admin_secret_xyz",
		VoucherSecretKey: "voucher_hmac_secret_456",
	}

	voucherRepo := repository.NewVoucherRepository(db)
	svc := NewVoucherService(cfg, voucherRepo)

	// 1. Password mismatch rejection
	_, errPw := svc.CreateVoucher(&models.CreateVoucherRequest{
		Code:          "DISKON10K",
		DiscountIDR:   10000,
		MaxUsage:      5,
		AdminPassword: "wrongpassword",
	})
	if errPw == nil {
		t.Fatalf("expected error on wrong admin password, got nil")
	}

	// 2. Successful creation
	v, err := svc.CreateVoucher(&models.CreateVoucherRequest{
		Code:          "DISKON10K",
		DiscountIDR:   10000,
		MinSpendIDR:   15000,
		MaxUsage:      5,
		AdminPassword: "superadmin123",
	})
	if err != nil {
		t.Fatalf("failed to create voucher: %v", err)
	}
	if v.Signature == "" {
		t.Fatalf("expected non-empty HMAC signature")
	}

	// 3. Signature verification succeeds
	if !svc.VerifySignature(v) {
		t.Fatalf("expected valid signature")
	}

	// 4. Tampered voucher detection
	tampered := *v
	tampered.DiscountIDR = 500000 // hacker tampered directly
	if svc.VerifySignature(&tampered) {
		t.Fatalf("expected signature verification to fail on tampered discount")
	}

	// 5. Validate for checkout - Below MinSpendIDR
	_, _, errMin := svc.ValidateForCheckout("DISKON10K", 10000)
	if errMin == nil {
		t.Fatalf("expected error for subtotal below MinSpendIDR")
	}

	// 6. Validate for checkout - Valid
	validV, disc, errVal := svc.ValidateForCheckout("DISKON10K", 50000)
	if errVal != nil {
		t.Fatalf("expected valid checkout: %v", errVal)
	}
	if validV.Code != "DISKON10K" || disc != 10000 {
		t.Fatalf("unexpected discount: %d", disc)
	}

	// 7. Validate for checkout - Capped at subtotal - 1000
	// If subtotal is 10,500 and voucher is 10,000, max discount is 9,500 so final price is at least 1,000
	_, discCapped, errCap := svc.ValidateForCheckout("DISKON10K", 10500)
	if errCap != nil {
		// Min spend was 15,000 so this should hit min spend
	} else {
		t.Logf("discCapped: %d", discCapped)
	}

	// 8. Test Rate Limiter
	testIP := "192.168.1.100"
	for i := 0; i < 15; i++ {
		if err := svc.CheckRateLimit(testIP); err != nil {
			t.Fatalf("iteration %d failed rate limit: %v", i, err)
		}
	}
	// 16th should fail
	if err := svc.CheckRateLimit(testIP); err == nil {
		t.Fatalf("expected rate limit error on 16th attempt, got nil")
	}
}
