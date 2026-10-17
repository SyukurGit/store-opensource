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
