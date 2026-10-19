package service

import (
	"context"
	"testing"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/provider"
	"storeku/internal/repository"
)

type mockProvider struct {
	id    string
	prods []*models.Product
	delay time.Duration
}

func (m *mockProvider) ID() string   { return m.id }
func (m *mockProvider) Name() string { return m.id }
func (m *mockProvider) FetchCatalog(ctx context.Context) ([]*models.Product, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.prods, nil
}
func (m *mockProvider) GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error) {
	return 5.0, nil
}
func (m *mockProvider) Fulfill(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error) {
	return &provider.FulfillmentResult{Success: true}, nil
}
func (m *mockProvider) CheckHealth(ctx context.Context) (*provider.HealthResult, error) {
	return &provider.HealthResult{Success: true, Balance: 100}, nil
}

func TestCatalogService_SWR_InstantResponse(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init memory db: %v", err)
	}

	reg := provider.NewRegistry()
	slowMock := &mockProvider{
		id: "MOCK_SLOW_SILVEX",
		prods: []*models.Product{
			{Slug: "mock-gemini", Name: "Mock Gemini Pro", PriceUSD: 5.0, ModalIDR: 75000, Stock: 10},
		},
		delay: 800 * time.Millisecond, // Simulates slow remote provider (like Silvex)
	}
	reg.Register(slowMock)

	svc := NewCatalogService(reg, nil, nil, nil, db)

	// Pre-populate in-memory cache directly (as if already synced or prewarmed)
	svc.cacheMu.Lock()
	svc.cachedProds = []*models.Product{
		{Slug: "instant-product", Name: "Instant Product", PriceIDR: 50000, Stock: 5},
	}
	svc.lastSyncTime = time.Now().Add(-3 * time.Minute) // Older than 2 minutes, triggers SWR refresh non-blockingly
	svc.cacheMu.Unlock()

	// Measure response time of GetPublicProducts: it MUST be instant and NOT wait 800ms!
	start := time.Now()
	prods, err := svc.GetPublicProducts(context.Background(), "")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(prods) == 0 {
		t.Fatalf("Expected products to be returned")
	}
	if prods[0].Slug != "instant-product" {
		t.Errorf("Expected 'instant-product', got '%s'", prods[0].Slug)
	}

	// Latency MUST be under 50ms, proving it did NOT block on the 800ms provider!
	if elapsed > 100*time.Millisecond {
		t.Errorf("GetPublicProducts blocked! Elapsed: %v, expected < 100ms", elapsed)
	} else {
		t.Logf("SUCCESS: SWR served instantaneously in %v (expected < 100ms, avoided 800ms delay)", elapsed)
	}
}

func TestCatalogService_PrewarmCache_NoDeadlock(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init memory db: %v", err)
	}

	// Insert test data into cached_products
	insertQuery := `
		INSERT INTO cached_products (slug, raw_id, name, type, logo, region, stock, stock_capped, warranty, description, price_usd, tiers_json)
		VALUES 
		('chatgpt-plus', '1', 'ChatGPT Plus', 'AI', '', 'Global', 10, 0, '30 Days', 'Test Desc', 20.0, '[]'),
		('claude-pro', '2', 'Claude Pro', 'AI', '', 'Global', 5, 0, '30 Days', 'Test Desc', 20.0, '[]'),
		('gemini-advanced', '3', 'Gemini Advanced', 'AI', '', 'Global', 15, 0, '30 Days', 'Test Desc', 19.99, '[]');
	`
	if _, err := db.Exec(insertQuery); err != nil {
		t.Fatalf("Failed to insert cached_products: %v", err)
	}

	// Insert setting to ensure setting table is queried
	_, _ = db.Exec(`INSERT INTO settings (key, value) VALUES ('usd_rate', '18000')`)

	settingRepo := repository.NewSettingRepository(db)
	pricing := NewPricingService(nil, settingRepo)
	reg := provider.NewRegistry()

	svc := NewCatalogService(reg, pricing, nil, settingRepo, db)

	done := make(chan bool, 1)
	go func() {
		svc.PrewarmCache()
		done <- true
	}()

	select {
	case <-done:
		// Succeeded without deadlock!
		svc.cacheMu.RLock()
		count := len(svc.cachedProds)
		svc.cacheMu.RUnlock()

		if count != 3 {
			t.Errorf("Expected 3 products loaded into memory, got %d", count)
		} else {
			t.Logf("SUCCESS: PrewarmCache loaded %d products with SetMaxOpenConns(1) with ZERO deadlock!", count)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("DEADLOCK DETECTED! PrewarmCache blocked for > 2s on SQLite connection pool!")
	}
}

func TestCatalogService_PromoPricingUSD_ReflectsPromoDiscount(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init memory db: %v", err)
	}
	defer db.Close()

	// Insert setting usd_rate = 18000
	_, _ = db.Exec(`INSERT INTO settings (key, value) VALUES ('usd_rate', '18000')`)
	// Insert override with promo price
	// Raw wholesale cost was $1.00 (Rp 18.000 modal)
	// Normal retail custom price = Rp 36.000 ($2.00)
	// Promo discount price = Rp 27.000 ($1.50)
	overrideQuery := `
		INSERT INTO product_overrides (product_slug, is_active, custom_price_idr, promo_price_idr, baseline_modal_idr)
		VALUES ('test-promo-prod', 1, 36000, 27000, 18000)
	`
	if _, err := db.Exec(overrideQuery); err != nil {
		t.Fatalf("Failed to insert override: %v", err)
	}

	cachedQuery := `
		INSERT INTO cached_products (slug, raw_id, name, type, logo, region, stock, stock_capped, warranty, description, price_usd, tiers_json)
		VALUES ('test-promo-prod', 'raw-1', 'Test Promo Product', 'License', '', 'Global', 10, 0, '1 Bulan', 'Desc', 1.0, '[]')
	`
	if _, err := db.Exec(cachedQuery); err != nil {
		t.Fatalf("Failed to insert cached product: %v", err)
	}

	settingRepo := repository.NewSettingRepository(db)
	overrideRepo := repository.NewProductOverrideRepository(db)
	pricing := NewPricingService(nil, settingRepo)
	reg := provider.NewRegistry()

	svc := NewCatalogService(reg, pricing, overrideRepo, settingRepo, db)

	ctx := context.Background()
	products, err := svc.GetPublicProducts(ctx, "id")
	if err != nil {
		t.Fatalf("GetPublicProducts failed: %v", err)
	}

	if len(products) != 1 {
		t.Fatalf("Expected 1 product, got %d", len(products))
	}

	p := products[0]
	if !p.HasPromo {
		t.Errorf("Expected HasPromo=true, got false")
	}

	// PriceIDR should be promo price (27000)
	if p.PriceIDR != 27000 {
		t.Errorf("Expected PriceIDR=27000, got %d", p.PriceIDR)
	}

	// ReferencePriceIDR should be 36000
	if p.ReferencePriceIDR != 36000 {
		t.Errorf("Expected ReferencePriceIDR=36000, got %d", p.ReferencePriceIDR)
	}

	// CRITICAL CHECK: In USD mode, PriceUSD MUST be 27000/18000 = $1.50, NOT the wholesale cost $1.00!
	expectedPriceUSD := 1.50
	if p.PriceUSD != expectedPriceUSD {
		t.Errorf("CRITICAL BUG: PriceUSD must be final promo price %.2f, but got %.2f (wholesale base price!)", expectedPriceUSD, p.PriceUSD)
	}

	// ReferencePriceUSD MUST be 36000/18000 = $2.00
	expectedRefUSD := 2.00
	if p.ReferencePriceUSD != expectedRefUSD {
		t.Errorf("ReferencePriceUSD must be %.2f, got %.2f", expectedRefUSD, p.ReferencePriceUSD)
	}

	// BasePriceUSD must preserve the raw wholesale cost $1.00
	if p.BasePriceUSD != 1.00 {
		t.Errorf("BasePriceUSD must preserve raw cost 1.00, got %.2f", p.BasePriceUSD)
	}

	t.Logf("SUCCESS: Promo Product in USD mode verified: Promo PriceUSD=$%.2f (was NOT reverted to base modal cost $%.2f), RefPriceUSD=$%.2f",
		p.PriceUSD, p.BasePriceUSD, p.ReferencePriceUSD)
}

func TestCatalogService_DisplayOrderPriorityOneOnTop(t *testing.T) {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to init memory db: %v", err)
	}
	defer db.Close()

	// Insert 4 cached products with different display orders & sales
	cachedQuery := `
		INSERT INTO cached_products (slug, raw_id, name, type, logo, region, stock, stock_capped, warranty, description, price_usd, tiers_json)
		VALUES
		('prod-a', '1', 'Apple Account', 'AI', '', 'Global', 10, 0, '1 Bulan', 'Desc', 5.0, '[]'),
		('prod-b', '2', 'Zebra License', 'AI', '', 'Global', 10, 0, '1 Bulan', 'Desc', 5.0, '[]'),
		('prod-c', '3', 'Banana Tool', 'AI', '', 'Global', 10, 0, '1 Bulan', 'Desc', 5.0, '[]'),
		('prod-d', '4', 'Zeta Account', 'AI', '', 'Global', 10, 0, '1 Bulan', 'Desc', 5.0, '[]');
	`
	if _, err := db.Exec(cachedQuery); err != nil {
		t.Fatalf("Failed to insert cached_products: %v", err)
	}

	// Overrides:
	// prod-b: DisplayOrder = 1, SalesCount = 10
	// prod-c: DisplayOrder = 2, SalesCount = 50
	// prod-a: DisplayOrder = 0, SalesCount = 100
	// prod-d: DisplayOrder = 0, SalesCount = 200
	overrideQuery := `
		INSERT INTO product_overrides (product_slug, is_active, display_order, sales_count)
		VALUES
		('prod-b', 1, 1, 10),
		('prod-c', 1, 2, 50),
		('prod-a', 1, 0, 100),
		('prod-d', 1, 0, 200);
	`
	if _, err := db.Exec(overrideQuery); err != nil {
		t.Fatalf("Failed to insert overrides: %v", err)
	}

	settingRepo := repository.NewSettingRepository(db)
	overrideRepo := repository.NewProductOverrideRepository(db)
	pricing := NewPricingService(nil, settingRepo)
	reg := provider.NewRegistry()

	svc := NewCatalogService(reg, pricing, overrideRepo, settingRepo, db)

	ctx := context.Background()
	products, err := svc.GetPublicProducts(ctx, "id")
	if err != nil {
		t.Fatalf("GetPublicProducts failed: %v", err)
	}

	if len(products) != 4 {
		t.Fatalf("Expected 4 products, got %d", len(products))
	}

	// Check 1: prod-b (Priority 1) MUST be first at index 0
	if products[0].Slug != "prod-b" {
		t.Fatalf("PRIORITY 1 INVERSION BUG: Expected 'prod-b' at top (index 0), got '%s' (DisplayOrder=%d)",
			products[0].Slug, products[0].DisplayOrder)
	}

	// Check 2: prod-c (Priority 2) MUST be second at index 1
	if products[1].Slug != "prod-c" {
		t.Fatalf("PRIORITY 2 MISPLACED: Expected 'prod-c' at index 1, got '%s'", products[1].Slug)
	}

	// Check 3: Default items (DisplayOrder=0) sort by SalesCount DESC -> prod-d (sales 200) then prod-a (sales 100)
	if products[2].Slug != "prod-d" {
		t.Fatalf("Expected 'prod-d' at index 2 (Sales 200), got '%s'", products[2].Slug)
	}
	if products[3].Slug != "prod-a" {
		t.Fatalf("Expected 'prod-a' at index 3 (Sales 100), got '%s'", products[3].Slug)
	}

	t.Logf("SUCCESS: Etalase order verified: 1st=%s (order %d), 2nd=%s (order %d), 3rd=%s (sales %d), 4th=%s (sales %d)",
		products[0].Slug, products[0].DisplayOrder,
		products[1].Slug, products[1].DisplayOrder,
		products[2].Slug, products[2].SalesCount,
		products[3].Slug, products[3].SalesCount)
}

func TestLocalProduct_StockAndUnlimitedModel(t *testing.T) {
	// 1. Unlimited local product
	lpUnlim := &models.LocalProduct{
		Slug:             "unlimited-item",
		Name:             "Unlimited Item",
		PriceIDR:         25000,
		Stock:            0,
		IsUnlimitedStock: true,
		IsActive:         true,
	}
	prodUnlim := lpUnlim.ToProduct()
	if prodUnlim.Stock != 9999 {
		t.Errorf("Expected unlimited product stock 9999, got %d", prodUnlim.Stock)
	}
	if !prodUnlim.StockCapped {
		t.Errorf("Expected StockCapped=true for unlimited, got false")
	}

	// 2. Custom manual stock local product
	lpManual := &models.LocalProduct{
		Slug:             "manual-stock-item",
		Name:             "Manual Stock Item",
		PriceIDR:         35000,
		Stock:            15,
		IsUnlimitedStock: false,
		IsActive:         true,
	}
	prodManual := lpManual.ToProduct()
	if prodManual.Stock != 15 {
		t.Errorf("Expected manual stock 15, got %d", prodManual.Stock)
	}
	if prodManual.StockCapped {
		t.Errorf("Expected StockCapped=false for manual stock, got true")
	}

	// 3. Force out of stock overrides unlimited
	lpForceOut := &models.LocalProduct{
		Slug:              "force-out-item",
		Name:              "Force Out Item",
		PriceIDR:          35000,
		Stock:             15,
		IsUnlimitedStock:  true,
		IsForceOutOfStock: true,
		IsActive:          true,
	}
	prodForceOut := lpForceOut.ToProduct()
	if prodForceOut.Stock != 0 {
		t.Errorf("Expected stock 0 when force out of stock, got %d", prodForceOut.Stock)
	}
}

func TestCatalogService_InvalidateCache_And_MemoryUpdate(t *testing.T) {
	service := &CatalogService{
		cachedProds: []*models.Product{
			{Slug: "test-prod", Name: "Test Product", SalesCount: 10, DisplayOrder: 0},
			{Slug: "other-prod", Name: "Other Product", SalesCount: 20, DisplayOrder: 1},
		},
		refreshTrigger: make(chan struct{}, 1),
	}

	// 1. Test instant memory update
	service.UpdateDisplayStatsMemory("test-prod", 5, 88)
	if service.cachedProds[0].SalesCount != 88 {
		t.Errorf("Expected SalesCount 88, got %d", service.cachedProds[0].SalesCount)
	}
	if service.cachedProds[0].DisplayOrder != 5 {
		t.Errorf("Expected DisplayOrder 5, got %d", service.cachedProds[0].DisplayOrder)
	}

	// 2. Test InvalidateCache clears cachedProds
	service.InvalidateCache()
	if service.cachedProds != nil {
		t.Errorf("Expected cachedProds to be nil after InvalidateCache, got len=%d", len(service.cachedProds))
	}
}
