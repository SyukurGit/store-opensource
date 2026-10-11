package service

import (
	"path/filepath"
	"testing"
	"time"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/repository"
)

func setupTestOrderService(t *testing.T) (*OrderService, *repository.OrderRepository) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	orderRepo := repository.NewOrderRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	overrideRepo := repository.NewProductOverrideRepository(db)
	cfg := &config.Config{ProfitMarginPercent: 25.0, USDToIDRRate: 18000.0}
	pricing := NewPricingService(cfg, settingRepo)
	dcClient := NewDigitalCoreClient(cfg, pricing, overrideRepo)
	pgClient := NewPaymentGClient(cfg)
	resend := NewResendService(cfg)

	orderService := NewOrderService(orderRepo, pricing, dcClient, pgClient, resend)
	return orderService, orderRepo
}

func TestCancelOrder(t *testing.T) {
	svc, repo := setupTestOrderService(t)

	// 1. Create a pending order
	exp := time.Now().Add(15 * time.Minute)
	pendingOrder := &models.Order{
		InvoiceNumber:     "INV-TEST-001",
		ClaimCode:         "CLM-TEST-001",
		ProductSlug:       "test-prod",
		ProductName:       "Test Product",
		Quantity:          1,
		TotalAmount:       50000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(pendingOrder); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 2. Cancel order
	if err := svc.CancelOrder("INV-TEST-001"); err != nil {
		t.Fatalf("expected cancel to succeed, got: %v", err)
	}

	// 3. Verify status is CANCELLED
	status, err := svc.GetOrderStatus("INV-TEST-001")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status.PaymentStatus != "CANCELLED" {
		t.Fatalf("expected status CANCELLED, got %s", status.PaymentStatus)
	}

	// 4. Test cancel on a PAID order should fail
	paidExp := time.Now().Add(15 * time.Minute)
	paidOrder := &models.Order{
		InvoiceNumber:     "INV-TEST-002",
		ClaimCode:         "CLM-TEST-002",
		ProductSlug:       "test-prod-paid",
		ProductName:       "Test Paid Product",
		Quantity:          1,
		TotalAmount:       75000,
		PaymentStatus:     "PAID",
		FulfillmentStatus: "SUCCESS",
		ExpiresAt:         &paidExp,
	}
	if err := repo.CreateOrder(paidOrder); err != nil {
		t.Fatalf("failed to create paid order: %v", err)
	}

	if err := svc.CancelOrder("INV-TEST-002"); err == nil {
		t.Fatalf("expected error cancelling PAID order, got nil")
	}
}

func TestAutoExpireOrder(t *testing.T) {
	svc, repo := setupTestOrderService(t)

	// 1. Create an expired pending order (expired 10 minutes ago)
	pastTime := time.Now().Add(-10 * time.Minute)
	order := &models.Order{
		InvoiceNumber:     "INV-EXPIRED-001",
		ClaimCode:         "CLM-EXPIRED-001",
		ProductSlug:       "test-prod-exp",
		ProductName:       "Test Expired Product",
		Quantity:          1,
		TotalAmount:       25000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &pastTime,
	}
	if err := repo.CreateOrder(order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 2. Calling GetOrderStatus should detect expiration and update to EXPIRED
	status, err := svc.GetOrderStatus("INV-EXPIRED-001")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status.PaymentStatus != "EXPIRED" {
		t.Fatalf("expected status EXPIRED, got %s", status.PaymentStatus)
	}

	// 3. Verify in DB
	dbOrder, err := repo.GetOrderByInvoice("INV-EXPIRED-001")
	if err != nil {
		t.Fatalf("failed to fetch from db: %v", err)
	}
	if dbOrder.PaymentStatus != "EXPIRED" {
		t.Fatalf("expected db status EXPIRED, got %s", dbOrder.PaymentStatus)
	}
}

func TestPriceSpikeAutoHoldAndRefund(t *testing.T) {
	svc, repo := setupTestOrderService(t)

	// 1. Create order where buyer paid 10,000 IDR
	exp := time.Now().Add(15 * time.Minute)
	order := &models.Order{
		InvoiceNumber:     "INV-SPIKE-001",
		ClaimCode:         "CLM-SPIKE-001",
		ProductSlug:       "gemini-test",
		ProductName:       "Google Gemini Advanced",
		Quantity:          1,
		TotalAmount:       10000,
		PaymentStatus:     "PAID",
		FulfillmentStatus: "HOLD_PRICE_SPIKE",
		FulfillmentError:  "Lonjakan harga modal upstream melebihi uang bayar",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 2. Customer checks order status on payment page
	status, err := svc.GetOrderStatus("INV-SPIKE-001")
	if err != nil {
		t.Fatalf("failed to get order status: %v", err)
	}
	if !status.IsQuotaOut {
		t.Fatalf("expected IsQuotaOut to be true for HOLD_PRICE_SPIKE")
	}
	if status.QuotaOutMessage == "" {
		t.Fatalf("expected QuotaOutMessage to not be empty")
	}

	// 3. Customer checks on claim page
	claim, err := svc.ClaimProduct("CLM-SPIKE-001")
	if err != nil {
		t.Fatalf("failed to claim: %v", err)
	}
	if !claim.IsQuotaOut {
		t.Fatalf("expected IsQuotaOut to be true on claim page")
	}
	if len(claim.DeliveredItems) != 0 {
		t.Fatalf("expected 0 delivered items for held order, got %d", len(claim.DeliveredItems))
	}

	// 4. Admin marks order as REFUNDED
	if err := svc.MarkRefundedOrder("INV-SPIKE-001"); err != nil {
		t.Fatalf("expected MarkRefundedOrder to succeed, got: %v", err)
	}

	dbOrder, err := repo.GetOrderByInvoice("INV-SPIKE-001")
	if err != nil {
		t.Fatalf("failed to fetch order: %v", err)
	}
	if dbOrder.FulfillmentStatus != "REFUNDED" {
		t.Fatalf("expected fulfillment_status REFUNDED, got %s", dbOrder.FulfillmentStatus)
	}
}

func TestSweepExpiredOrders(t *testing.T) {
	_, repo := setupTestOrderService(t)

	past := time.Now().Add(-30 * time.Minute)
	future := time.Now().Add(10 * time.Minute)

	// Create 1 expired and 1 active pending order
	_ = repo.CreateOrder(&models.Order{
		InvoiceNumber:     "INV-SWEEP-001",
		ClaimCode:         "CLM-SWEEP-001",
		ProductSlug:       "p1",
		ProductName:       "Prod 1",
		Quantity:          1,
		TotalAmount:       10000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &past,
	})
	_ = repo.CreateOrder(&models.Order{
		InvoiceNumber:     "INV-SWEEP-002",
		ClaimCode:         "CLM-SWEEP-002",
		ProductSlug:       "p2",
		ProductName:       "Prod 2",
		Quantity:          1,
		TotalAmount:       20000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &future,
	})

	affected, err := repo.SweepExpiredOrders()
	if err != nil {
		t.Fatalf("SweepExpiredOrders failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 expired order swept, got %d", affected)
	}

	o1, _ := repo.GetOrderByInvoice("INV-SWEEP-001")
	if o1.PaymentStatus != "EXPIRED" {
		t.Fatalf("expected INV-SWEEP-001 to be EXPIRED, got %s", o1.PaymentStatus)
	}

	o2, _ := repo.GetOrderByInvoice("INV-SWEEP-002")
	if o2.PaymentStatus != "PENDING" {
		t.Fatalf("expected INV-SWEEP-002 to remain PENDING, got %s", o2.PaymentStatus)
	}
}

func TestManualMarkPaidAndFulfill(t *testing.T) {
	svc, repo := setupTestOrderService(t)

	exp := time.Now().Add(15 * time.Minute)
	_ = repo.CreateOrder(&models.Order{
		InvoiceNumber:     "INV-MANUAL-001",
		ClaimCode:         "CLM-MANUAL-001",
		ProductSlug:       "p-manual",
		ProductName:       "Manual Product",
		Quantity:          1,
		TotalAmount:       50000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	})

	// Marking paid manually should update payment status to PAID
	_ = svc.ManualMarkPaidAndFulfill("INV-MANUAL-001")

	o, err := repo.GetOrderByInvoice("INV-MANUAL-001")
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}
	if o.PaymentStatus != "PAID" {
		t.Fatalf("expected payment_status PAID, got %s", o.PaymentStatus)
	}
}

func TestProductOverrideForceOutOfStockRepo(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	defer db.Close()

	repo := repository.NewProductOverrideRepository(db)
	err = repo.Upsert(&models.ProductOverride{
		ProductSlug:       "netflix-1m",
		CustomName:        "Netflix 1 Bulan",
		IsActive:          true,
		IsForceOutOfStock: true,
	})
	if err != nil {
		t.Fatalf("failed to upsert override: %v", err)
	}

	ov, err := repo.GetBySlug("netflix-1m")
	if err != nil {
		t.Fatalf("failed to get override: %v", err)
	}
	if ov == nil || !ov.IsForceOutOfStock {
		t.Fatalf("expected IsForceOutOfStock true, got false")
	}
	if !ov.IsActive {
		t.Fatalf("expected IsActive true, got false")
	}

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("failed to get all overrides: %v", err)
	}
	if all["netflix-1m"] == nil || !all["netflix-1m"].IsForceOutOfStock {
		t.Fatalf("expected GetAll to contain IsForceOutOfStock true")
	}
}

func TestRefundedOrderProtection(t *testing.T) {
	svc, repo := setupTestOrderService(t)

	exp := time.Now().Add(15 * time.Minute)
	order := &models.Order{
		InvoiceNumber:     "INV-REFUND-001",
		ClaimCode:         "CLM-REFUND-001",
		ProductSlug:       "test-prod",
		ProductName:       "Test Product",
		Quantity:          1,
		TotalAmount:       50000,
		PaymentStatus:     "PAID",
		FulfillmentStatus: "FAILED",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 1. Mark as refunded
	if err := svc.MarkRefundedOrder("INV-REFUND-001"); err != nil {
		t.Fatalf("expected MarkRefundedOrder to succeed, got: %v", err)
	}

	o, err := repo.GetOrderByInvoice("INV-REFUND-001")
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}
	if o.PaymentStatus != "REFUNDED" || o.FulfillmentStatus != "REFUNDED" {
		t.Fatalf("expected REFUNDED status, got payment=%s fulfillment=%s", o.PaymentStatus, o.FulfillmentStatus)
	}

	// 2. RetryFulfillment must fail
	if err := svc.RetryFulfillment("INV-REFUND-001"); err == nil {
		t.Fatalf("expected RetryFulfillment to fail on REFUNDED order, got nil")
	}

	// 3. ForceFulfillOrder must fail
	if err := svc.ForceFulfillOrder("INV-REFUND-001"); err == nil {
		t.Fatalf("expected ForceFulfillOrder to fail on REFUNDED order, got nil")
	}
}


