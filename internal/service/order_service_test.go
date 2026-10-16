package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/provider"
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

func TestDigitalCoreCheckConnectionAndParse(t *testing.T) {
	cfg := &config.Config{
		DigitalCoreBaseURL: "https://digitalcore.top",
		DigitalCoreAPIKey:  "",
	}
	client := NewDigitalCoreClient(cfg, nil, nil)

	// 1. When empty, check fails gracefully
	res, err := client.CheckConnection()
	if err == nil {
		t.Fatalf("expected error when API key is empty, got nil")
	}
	if res.Configured {
		t.Fatalf("expected Configured to be false")
	}

	// 2. Test JSON parsing with top level balance
	jsonTop := []byte(`{"balance": 25.50, "total_orders": 12}`)
	bal, orders, err := parseDigitalCoreUserMe(jsonTop)
	if err != nil || bal != 25.50 || orders != "12" {
		t.Fatalf("failed parsing top-level json: bal=%f orders=%s err=%v", bal, orders, err)
	}

	// 3. Test JSON parsing with nested data & string balance
	jsonNested := []byte(`{"success": true, "data": {"balance": "14.25", "total_orders": "8"}}`)
	bal, orders, err = parseDigitalCoreUserMe(jsonNested)
	if err != nil || bal != 14.25 || orders != "8" {
		t.Fatalf("failed parsing nested json: bal=%f orders=%s err=%v", bal, orders, err)
	}
}

func TestProductPricingCustomAndPromoOverridesTiers(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	overrideRepo := repository.NewProductOverrideRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	cfg := &config.Config{ProfitMarginPercent: 25.0, USDToIDRRate: 18000.0}
	pricing := NewPricingService(cfg, settingRepo)
	dcClient := NewDigitalCoreClient(cfg, pricing, overrideRepo)

	// Setup cached raw products
	rawP1 := &models.Product{
		Slug:     "prod-custom",
		Name:     "Custom Product",
		PriceUSD: 1.0,
		PriceIDR: 22500, // 1 * 18000 * 1.25 = 22500
		Stock:    10,
		Tiers: []models.ProductTier{
			{MinQty: 1, MaxQty: nil, PriceUSD: 0.9, PriceIDR: 20250},
		},
	}
	rawP2 := &models.Product{
		Slug:     "prod-normal",
		Name:     "Normal Product",
		PriceUSD: 2.0,
		PriceIDR: 45000,
		Stock:    5,
		Tiers: []models.ProductTier{
			{MinQty: 1, MaxQty: nil, PriceUSD: 1.8, PriceIDR: 40500},
		},
	}
	dcClient.cachedProds = []*models.Product{rawP1, rawP2}
	dcClient.cacheExpires = time.Now().Add(1 * time.Hour)

	// Save override for prod-custom
	err = overrideRepo.Upsert(&models.ProductOverride{
		ProductSlug:      "prod-custom",
		CustomPriceIDR:   35000,
		BaselineModalIDR: 18000,
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("failed to save override: %v", err)
	}

	prods, err := dcClient.FetchProducts()
	if err != nil {
		t.Fatalf("FetchProducts failed: %v", err)
	}

	var foundCustom, foundNormal *models.Product
	for _, p := range prods {
		if p.Slug == "prod-custom" {
			foundCustom = p
		} else if p.Slug == "prod-normal" {
			foundNormal = p
		}
	}

	if foundCustom == nil || foundNormal == nil {
		t.Fatalf("expected both products returned")
	}

	// Customized product should have CustomPriceIDR as PriceIDR and Tiers cleared
	if foundCustom.PriceIDR != 35000 {
		t.Fatalf("expected customized price 35000, got %d", foundCustom.PriceIDR)
	}
	if len(foundCustom.Tiers) != 0 {
		t.Fatalf("expected tiers to be cleared for customized product, got %d tiers", len(foundCustom.Tiers))
	}
	if !foundCustom.IsCustomized {
		t.Fatalf("expected IsCustomized to be true")
	}

	// Normal product should retain its tiers and default price
	if foundNormal.PriceIDR != 45000 {
		t.Fatalf("expected normal price 45000, got %d", foundNormal.PriceIDR)
	}
	if len(foundNormal.Tiers) != 1 {
		t.Fatalf("expected normal tiers preserved, got %d", len(foundNormal.Tiers))
	}

	// Verify that raw cached products were NOT mutated
	if rawP1.PriceIDR != 22500 || len(rawP1.Tiers) != 1 {
		t.Fatalf("raw cached product was unexpectedly mutated: price=%d tiers=%d", rawP1.PriceIDR, len(rawP1.Tiers))
	}
}

func TestDigitalCoreBuyProductPayloadAndURL(t *testing.T) {
	var capturedURL string
	var capturedHeaderKey string
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		capturedHeaderKey = r.Header.Get("Api-Key")

		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"order_id": "ORD-TEST-123",
			"view_url": "https://digitalcore.top/order?number=test",
			"total_amount": 0.30,
			"quantity": 1,
			"items": ["https://redeem.adobe.com/CODE123"]
		}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		DigitalCoreBaseURL: server.URL,
		DigitalCoreAPIKey:  "test-secret-key-123",
	}
	client := NewDigitalCoreClient(cfg, nil, nil)

	// Buy using slug with dash or mixed case
	resp, err := client.BuyProduct("adobe-express-4-months-trial", 1)
	if err != nil {
		t.Fatalf("expected buy to succeed, got error: %v", err)
	}

	if resp.OrderID != "ORD-TEST-123" {
		t.Fatalf("unexpected order id: %s", resp.OrderID)
	}

	if capturedHeaderKey != "test-secret-key-123" {
		t.Fatalf("expected Api-Key header to be passed, got: %s", capturedHeaderKey)
	}

	// Verify query string contains id and count
	if capturedURL != "/api/user/buy?id=adobe_express_4_months_trial&count=1" {
		t.Fatalf("unexpected URL: %s", capturedURL)
	}

	// Verify body also contains id and count
	if capturedBody["id"] != "adobe_express_4_months_trial" {
		t.Fatalf("expected body id to be adobe_express_4_months_trial, got: %v", capturedBody["id"])
	}
	if capturedBody["count"] != float64(1) {
		t.Fatalf("expected body count to be 1, got: %v", capturedBody["count"])
	}
}

func TestClaimCodeHiddenUntilPaid(t *testing.T) {
	svc, repo := setupTestOrderService(t)

	exp := time.Now().Add(15 * time.Minute)
	pendingOrder := &models.Order{
		InvoiceNumber:     "INV-CLAIM-TEST-001",
		ClaimCode:         "CLM-SECRET-123",
		ProductSlug:       "test-slug",
		ProductName:       "Test Product",
		Quantity:          1,
		TotalAmount:       50000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := repo.CreateOrder(pendingOrder); err != nil {
		t.Fatalf("failed to create pending order: %v", err)
	}

	// 1. Unpaid order status should NOT expose ClaimCode
	status, err := svc.GetOrderStatus("INV-CLAIM-TEST-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.ClaimCode != "" {
		t.Fatalf("expected ClaimCode to be empty for PENDING order, got: '%s'", status.ClaimCode)
	}

	// 2. Querying ClaimProduct by invoice while unpaid should NOT expose ClaimCode
	claimRes, err := svc.ClaimProduct("INV-CLAIM-TEST-001")
	if err != nil {
		t.Fatalf("unexpected error from ClaimProduct: %v", err)
	}
	if claimRes.ClaimCode != "" {
		t.Fatalf("expected ClaimCode to be empty from ClaimProduct for PENDING order, got: '%s'", claimRes.ClaimCode)
	}

	// 3. Update to PAID
	now := time.Now().UTC()
	if err := repo.UpdatePaymentStatus("INV-CLAIM-TEST-001", "PAID", &now); err != nil {
		t.Fatalf("failed to mark paid: %v", err)
	}

	// 4. Now GetOrderStatus should reveal ClaimCode
	paidStatus, err := svc.GetOrderStatus("INV-CLAIM-TEST-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paidStatus.ClaimCode != "CLM-SECRET-123" {
		t.Fatalf("expected ClaimCode 'CLM-SECRET-123' for PAID order, got: '%s'", paidStatus.ClaimCode)
	}

	// 5. Querying ClaimProduct should now reveal ClaimCode
	paidClaimRes, err := svc.ClaimProduct("CLM-SECRET-123")
	if err != nil {
		t.Fatalf("unexpected error from ClaimProduct: %v", err)
	}
	if paidClaimRes.ClaimCode != "CLM-SECRET-123" {
		t.Fatalf("expected ClaimCode 'CLM-SECRET-123' from ClaimProduct, got: '%s'", paidClaimRes.ClaimCode)
	}
}

func TestLocalProductOrderFulfillmentAndManualDeliver(t *testing.T) {
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
	localProdRepo := repository.NewLocalProductRepository(db)
	cfg := &config.Config{ProfitMarginPercent: 25.0, USDToIDRRate: 18000.0}
	pricing := NewPricingService(cfg, settingRepo)
	dcClient := NewDigitalCoreClient(cfg, pricing, overrideRepo)
	pgClient := NewPaymentGClient(cfg)
	resend := NewResendService(cfg)

	svc := NewOrderService(orderRepo, pricing, dcClient, pgClient, resend)
	svc.SetLocalProductRepo(localProdRepo)

	// 1. Create a local product with 2 keys
	pWithKeys, err := localProdRepo.Create(&models.LocalProductRequest{
		Name:      "Local Canva Pro",
		Slug:      "local-canva-pro",
		PriceIDR:  35000,
		IsActive:  true,
		StockKeys: []string{"KEY-CANVA-1", "KEY-CANVA-2"},
	})
	if err != nil {
		t.Fatalf("failed to create local product: %v", err)
	}

	// 2. Create local product with 0 keys (manual CS delivery)
	pNoKeys, err := localProdRepo.Create(&models.LocalProductRequest{
		Name:      "Local Netflix 0 Keys",
		Slug:      "local-netflix-0",
		PriceIDR:  45000,
		IsActive:  true,
		StockKeys: []string{},
	})
	if err != nil {
		t.Fatalf("failed to create local product 0 keys: %v", err)
	}

	// Scenario A: Order for pWithKeys -> auto fulfilled
	exp := time.Now().Add(15 * time.Minute)
	orderA := &models.Order{
		InvoiceNumber:     "INV-LOCAL-001",
		ClaimCode:         "CLM-LOCAL-001",
		ProductSlug:       pWithKeys.Slug,
		ProductName:       pWithKeys.Name,
		Quantity:          1,
		TotalAmount:       35000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := orderRepo.CreateOrder(orderA); err != nil {
		t.Fatalf("failed to create orderA: %v", err)
	}

	// Process payment success
	if err := svc.ProcessPaymentSuccess("INV-LOCAL-001", "TRX-LOCAL-001"); err != nil {
		t.Fatalf("ProcessPaymentSuccess failed: %v", err)
	}

	updatedA, err := orderRepo.GetOrderByInvoice("INV-LOCAL-001")
	if err != nil {
		t.Fatalf("failed to get orderA: %v", err)
	}
	if updatedA.PaymentStatus != "PAID" {
		t.Errorf("expected PAID, got %s", updatedA.PaymentStatus)
	}
	if updatedA.FulfillmentStatus != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", updatedA.FulfillmentStatus)
	}
	var deliveredKeysA []string
	_ = json.Unmarshal([]byte(updatedA.DeliveredItems), &deliveredKeysA)
	if len(deliveredKeysA) != 1 || deliveredKeysA[0] != "KEY-CANVA-1" {
		t.Errorf("expected KEY-CANVA-1 delivered, got %v", deliveredKeysA)
	}

	// Sisa key harus 1
	pAfterA, _ := localProdRepo.GetByID(pWithKeys.ID)
	if len(pAfterA.StockKeys) != 1 || pAfterA.StockKeys[0] != "KEY-CANVA-2" {
		t.Errorf("expected 1 key remaining (KEY-CANVA-2), got %v", pAfterA.StockKeys)
	}

	// Scenario B: Order for pNoKeys -> PENDING_MANUAL_DELIVERY
	orderB := &models.Order{
		InvoiceNumber:     "INV-LOCAL-002",
		ClaimCode:         "CLM-LOCAL-002",
		ProductSlug:       pNoKeys.Slug,
		ProductName:       pNoKeys.Name,
		Quantity:          1,
		TotalAmount:       45000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := orderRepo.CreateOrder(orderB); err != nil {
		t.Fatalf("failed to create orderB: %v", err)
	}

	if err := svc.ProcessPaymentSuccess("INV-LOCAL-002", "TRX-LOCAL-002"); err != nil {
		t.Fatalf("ProcessPaymentSuccess failed for orderB: %v", err)
	}

	updatedB, err := orderRepo.GetOrderByInvoice("INV-LOCAL-002")
	if err != nil {
		t.Fatalf("failed to get orderB: %v", err)
	}
	if updatedB.PaymentStatus != "PAID" {
		t.Errorf("expected PAID for orderB, got %s", updatedB.PaymentStatus)
	}
	if updatedB.FulfillmentStatus != "PENDING_MANUAL_DELIVERY" {
		t.Errorf("expected PENDING_MANUAL_DELIVERY for orderB, got %s", updatedB.FulfillmentStatus)
	}

	// Now Admin delivers keys manually
	manualKeys := []string{"MANUAL-KEY-NETFLIX-999"}
	if err := svc.ManualDeliverKeys("INV-LOCAL-002", manualKeys); err != nil {
		t.Fatalf("ManualDeliverKeys failed: %v", err)
	}

	fulfilledB, err := orderRepo.GetOrderByInvoice("INV-LOCAL-002")
	if err != nil {
		t.Fatalf("failed to get fulfilledB: %v", err)
	}
	if fulfilledB.FulfillmentStatus != "SUCCESS" {
		t.Errorf("expected SUCCESS after manual deliver, got %s", fulfilledB.FulfillmentStatus)
	}
	var deliveredKeysB []string
	_ = json.Unmarshal([]byte(fulfilledB.DeliveredItems), &deliveredKeysB)
	if len(deliveredKeysB) != 1 || deliveredKeysB[0] != "MANUAL-KEY-NETFLIX-999" {
		t.Errorf("expected MANUAL-KEY-NETFLIX-999 delivered, got %v", deliveredKeysB)
	}
}

type mockSilvexProvider struct {
	fulfillFunc func(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error)
	liveCost    float64
}

func (m *mockSilvexProvider) ID() string   { return "SILVEX" }
func (m *mockSilvexProvider) Name() string { return "Silvex AI" }
func (m *mockSilvexProvider) FetchCatalog(ctx context.Context) ([]*models.Product, error) {
	return []*models.Product{
		{Slug: "silvex_chatgpt_plus", Name: "ChatGPT Plus 1 Month", PriceUSD: 3.5, PriceIDR: 70000, Source: "SILVEX"},
	}, nil
}
func (m *mockSilvexProvider) GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error) {
	return m.liveCost, nil
}
func (m *mockSilvexProvider) Fulfill(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error) {
	if m.fulfillFunc != nil {
		return m.fulfillFunc(ctx, req)
	}
	return &provider.FulfillmentResult{
		Success:          true,
		ProviderOrderID:  "SILVEX-ORD-888",
		DeliveredItems:   []string{"Account: test@example.com:password123"},
		WholesaleCostUSD: 3.5,
	}, nil
}
func (m *mockSilvexProvider) CheckHealth(ctx context.Context) (*provider.HealthResult, error) {
	return &provider.HealthResult{Success: true, Configured: true, Balance: 50.0, Currency: "USD"}, nil
}

func TestMultiProviderSilvexOrderFulfillment(t *testing.T) {
	svc, orderRepo := setupTestOrderService(t)

	mockSilvex := &mockSilvexProvider{
		liveCost: 3.5,
	}
	reg := provider.NewRegistry()
	reg.Register(mockSilvex)
	svc.SetProviderRegistry(reg)

	// Case 1: Instant Silvex fulfillment
	exp := time.Now().Add(15 * time.Minute)
	order1 := &models.Order{
		InvoiceNumber:     "INV-SILVEX-001",
		ClaimCode:         "CLM-SILVEX-001",
		ProductSlug:       "silvex_chatgpt_plus",
		ProductName:       "ChatGPT Plus 1 Month",
		ProductType:       "AI Account",
		Quantity:          1,
		TotalAmount:       85000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := orderRepo.CreateOrder(order1); err != nil {
		t.Fatalf("failed to create order1: %v", err)
	}

	if err := svc.ProcessPaymentSuccess("INV-SILVEX-001", "TRX-SILVEX-001"); err != nil {
		t.Fatalf("ProcessPaymentSuccess failed for silvex order1: %v", err)
	}

	updated1, err := orderRepo.GetOrderByInvoice("INV-SILVEX-001")
	if err != nil {
		t.Fatalf("failed to get order1: %v", err)
	}
	if updated1.PaymentStatus != "PAID" {
		t.Errorf("expected PAID, got %s", updated1.PaymentStatus)
	}
	if updated1.FulfillmentStatus != "SUCCESS" {
		t.Errorf("expected SUCCESS for instant Silvex delivery, got %s", updated1.FulfillmentStatus)
	}
	if updated1.DigitalCoreOrderID != "SILVEX-ORD-888" {
		t.Errorf("expected DigitalCoreOrderID 'SILVEX-ORD-888', got %s", updated1.DigitalCoreOrderID)
	}
	var deliveredItems1 []string
	_ = json.Unmarshal([]byte(updated1.DeliveredItems), &deliveredItems1)
	if len(deliveredItems1) != 1 || deliveredItems1[0] != "Account: test@example.com:password123" {
		t.Errorf("unexpected delivered items: %v", deliveredItems1)
	}

	// Case 2: Manual / Pending delivery from Silvex upstream
	mockSilvex.fulfillFunc = func(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error) {
		return &provider.FulfillmentResult{
			Success:          false,
			ProviderOrderID:  "SILVEX-PENDING-999",
			DeliveredItems:   []string{},
			ErrorMessage:     "PENDING_MANUAL_DELIVERY",
			WholesaleCostUSD: 3.5,
		}, nil
	}

	order2 := &models.Order{
		InvoiceNumber:     "INV-SILVEX-002",
		ClaimCode:         "CLM-SILVEX-002",
		ProductSlug:       "silvex_chatgpt_plus",
		ProductName:       "ChatGPT Plus 1 Month",
		ProductType:       "AI Account",
		Quantity:          1,
		TotalAmount:       85000,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		ExpiresAt:         &exp,
	}
	if err := orderRepo.CreateOrder(order2); err != nil {
		t.Fatalf("failed to create order2: %v", err)
	}

	if err := svc.ProcessPaymentSuccess("INV-SILVEX-002", "TRX-SILVEX-002"); err != nil {
		t.Fatalf("ProcessPaymentSuccess failed for silvex order2: %v", err)
	}

	updated2, err := orderRepo.GetOrderByInvoice("INV-SILVEX-002")
	if err != nil {
		t.Fatalf("failed to get order2: %v", err)
	}
	if updated2.PaymentStatus != "PAID" {
		t.Errorf("expected PAID, got %s", updated2.PaymentStatus)
	}
	if updated2.FulfillmentStatus != "PENDING_MANUAL_DELIVERY" {
		t.Errorf("expected PENDING_MANUAL_DELIVERY for manual Silvex order, got %s", updated2.FulfillmentStatus)
	}
	if updated2.DigitalCoreOrderID != "SILVEX-PENDING-999" {
		t.Errorf("expected DigitalCoreOrderID 'SILVEX-PENDING-999', got %s", updated2.DigitalCoreOrderID)
	}
}

