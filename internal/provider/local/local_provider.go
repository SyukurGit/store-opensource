package local

import (
	"context"
	"fmt"

	"storeku/internal/models"
	"storeku/internal/provider"
	"storeku/internal/repository"
)

// LocalProvider implements provider.Provider for internal standalone inventory.
type LocalProvider struct {
	repo *repository.LocalProductRepository
}

// NewLocalProvider creates a new LocalProvider adapter.
func NewLocalProvider(repo *repository.LocalProductRepository) *LocalProvider {
	return &LocalProvider{
		repo: repo,
	}
}

// ID returns the unique identifier for local inventory.
func (p *LocalProvider) ID() string {
	return "LOCAL"
}

// Name returns the display name for local inventory.
func (p *LocalProvider) Name() string {
	return "Stok Lokal Mandiri"
}

// FetchCatalog returns all active local products normalized to canonical models.Product.
func (p *LocalProvider) FetchCatalog(ctx context.Context) ([]*models.Product, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("local product repository is not initialized")
	}

	activeItems, err := p.repo.GetAllActive()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch local products: %w", err)
	}

	products := make([]*models.Product, 0, len(activeItems))
	for _, item := range activeItems {
		products = append(products, item.ToProduct())
	}

	return products, nil
}

// GetLiveCost for local products is zero (stored internal stock).
func (p *LocalProvider) GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error) {
	return 0, nil
}

// Fulfill claims serial keys atomically from the local database pool.
func (p *LocalProvider) Fulfill(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("local product repository is not initialized")
	}

	claimedKeys, err := p.repo.ClaimKeys(req.ProductSlug, req.Quantity)
	if err != nil {
		return &provider.FulfillmentResult{
			Success:         false,
			ProviderOrderID: "LOCAL-ORDER",
			DeliveredItems:  []string{},
			ErrorMessage:    err.Error(),
		}, err
	}

	if len(claimedKeys) > 0 {
		return &provider.FulfillmentResult{
			Success:          true,
			ProviderOrderID:  "LOCAL-ORDER",
			DeliveredItems:   claimedKeys,
			WholesaleCostUSD: 0,
			WholesaleCostIDR: 0,
		}, nil
	}

	// No keys left in stock: signal manual delivery
	return &provider.FulfillmentResult{
		Success:          false,
		ProviderOrderID:  "LOCAL-ORDER",
		DeliveredItems:   []string{},
		ErrorMessage:     "PENDING_MANUAL_DELIVERY",
		WholesaleCostUSD: 0,
		WholesaleCostIDR: 0,
	}, nil
}

// CheckHealth returns the operational status of the local product repository.
func (p *LocalProvider) CheckHealth(ctx context.Context) (*provider.HealthResult, error) {
	if p.repo == nil {
		return &provider.HealthResult{
			Success:      false,
			Configured:   false,
			ProviderID:   p.ID(),
			ProviderName: p.Name(),
			Message:      "Repository lokal belum diinisialisasi",
		}, fmt.Errorf("repository lokal nil")
	}

	prods, err := p.repo.GetAll()
	if err != nil {
		return &provider.HealthResult{
			Success:      false,
			Configured:   true,
			ProviderID:   p.ID(),
			ProviderName: p.Name(),
			Message:      fmt.Sprintf("Gagal membaca database lokal: %v", err),
		}, err
	}

	return &provider.HealthResult{
		Success:      true,
		Configured:   true,
		ProviderID:   p.ID(),
		ProviderName: p.Name(),
		Currency:     "IDR",
		TotalOrders:  fmt.Sprintf("%d produk", len(prods)),
		Message:      fmt.Sprintf("Database produk lokal aktif (%d item terdaftar)", len(prods)),
	}, nil
}
