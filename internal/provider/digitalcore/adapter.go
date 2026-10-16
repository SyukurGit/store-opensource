package digitalcore

import (
	"context"
	"fmt"
	"strings"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/provider"
)

// Adapter wraps the DigitalCore Client to implement provider.Provider.
type Adapter struct {
	client  *Client
	pricing PricingCalculator
	cfg     *config.Config
}

// NewAdapter creates a new DigitalCore provider adapter.
func NewAdapter(cfg *config.Config, pricing PricingCalculator) *Adapter {
	return &Adapter{
		client:  NewClient(cfg),
		pricing: pricing,
		cfg:     cfg,
	}
}

// Client returns the underlying HTTP client for direct queries if needed.
func (a *Adapter) Client() *Client {
	return a.client
}

// ID returns the unique system identifier for DigitalCore.
func (a *Adapter) ID() string {
	return "DIGITALCORE"
}

// Name returns the human-readable display name for DigitalCore.
func (a *Adapter) Name() string {
	return "DigitalCore Upstream"
}

// FetchCatalog queries DigitalCore and normalizes the active catalog.
func (a *Adapter) FetchCatalog(ctx context.Context) ([]*models.Product, error) {
	rawProds, err := a.client.FetchRawProducts(ctx)
	if err != nil {
		return nil, err
	}

	var prods []*models.Product
	for _, p := range rawProds {
		if strings.ToLower(p.Status) != "active" {
			continue
		}
		prods = append(prods, NormalizeRawProduct(p, a.client.getBaseURL(), a.pricing))
	}

	return prods, nil
}

// GetLiveCost queries the real-time wholesale USD cost for a product and quantity.
func (a *Adapter) GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error) {
	rawProds, err := a.client.FetchRawProducts(ctx)
	if err != nil {
		return 0, err
	}

	target := strings.TrimSpace(itemID)
	for _, p := range rawProds {
		pSlug := ToSlug(p.Name)
		if strings.EqualFold(pSlug, target) || strings.EqualFold(fmt.Sprintf("%v", p.ID), target) || strings.EqualFold(p.Name, target) {
			unitCostUSD := p.PriceFrom
			for _, tier := range p.Tiers {
				if qty >= tier.MinQty && (tier.MaxQty == nil || qty <= *tier.MaxQty) {
					unitCostUSD = tier.Price
					break
				}
			}
			return unitCostUSD, nil
		}
	}

	return 0, fmt.Errorf("produk '%s' tidak ditemukan di upstream DigitalCore", itemID)
}

// Fulfill buys the product on DigitalCore and formats the result.
func (a *Adapter) Fulfill(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error) {
	targetItem := req.ProviderItemID
	if targetItem == "" {
		targetItem = req.ProductSlug
	}

	buyResp, err := a.client.BuyProduct(ctx, targetItem, req.Quantity)
	if err != nil {
		return &provider.FulfillmentResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	return &provider.FulfillmentResult{
		Success:          buyResp.Success,
		ProviderOrderID:  buyResp.OrderID,
		ProviderViewURL:  buyResp.ViewURL,
		DeliveredItems:   buyResp.Items,
		WholesaleCostUSD: buyResp.TotalAmount,
	}, nil
}

// CheckHealth verifies connection and balance on DigitalCore.
func (a *Adapter) CheckHealth(ctx context.Context) (*provider.HealthResult, error) {
	return a.client.CheckConnection(ctx)
}

// GetUserAccount returns bot balance and order counts.
func (a *Adapter) GetUserAccount(ctx context.Context) (*UserMeResponse, error) {
	return a.client.GetUserAccount(ctx)
}
