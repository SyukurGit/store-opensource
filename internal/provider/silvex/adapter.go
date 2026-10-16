package silvex

import (
	"context"
	"fmt"
	"strings"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/provider"
)

// Adapter connects Silvex AI Reseller API to the StoreKu multi-provider system.
type Adapter struct {
	client  *Client
	pricing PricingCalculator
	cfg     *config.Config
}

// NewAdapter creates a new Silvex AI provider adapter.
func NewAdapter(cfg *config.Config, pricing PricingCalculator) *Adapter {
	return &Adapter{
		client:  NewClient(cfg),
		pricing: pricing,
		cfg:     cfg,
	}
}

// Client returns the underlying HTTP client.
func (a *Adapter) Client() *Client {
	return a.client
}

// ID returns the unique system identifier for Silvex AI.
func (a *Adapter) ID() string {
	return "SILVEX"
}

// Name returns the human-readable display name.
func (a *Adapter) Name() string {
	return "Silvex AI Reseller"
}

// FetchCatalog queries Silvex AI and returns canonical models.Product items.
func (a *Adapter) FetchCatalog(ctx context.Context) ([]*models.Product, error) {
	rawProds, err := a.client.GetProducts(ctx)
	if err != nil {
		return nil, err
	}

	var prods []*models.Product
	for _, p := range rawProds {
		if p.Status != "" && strings.ToLower(p.Status) != "active" {
			continue
		}
		prods = append(prods, NormalizeProduct(p, a.pricing))
	}

	return prods, nil
}

// GetLiveCost queries the live wholesale USD cost for a product.
func (a *Adapter) GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error) {
	rawProds, err := a.client.GetProducts(ctx)
	if err != nil {
		return 0, err
	}

	target := strings.TrimSpace(itemID)
	targetClean := strings.TrimPrefix(target, "silvex_")

	for _, p := range rawProds {
		rawID := fmt.Sprintf("%v", p.ID)
		if p.ProductID != "" {
			rawID = p.ProductID
		}
		pName := strings.ToLower(strings.ReplaceAll(p.Name, " ", "_"))

		if strings.EqualFold(rawID, target) || strings.EqualFold(pName, target) || strings.EqualFold(pName, targetClean) {
			price := p.Price
			if price <= 0 && p.PriceUSD > 0 {
				price = p.PriceUSD
			}
			return price, nil
		}
	}

	return 0, fmt.Errorf("produk '%s' tidak ditemukan di Silvex AI", itemID)
}

// Fulfill executes the purchase transaction on Silvex AI.
func (a *Adapter) Fulfill(ctx context.Context, req *provider.FulfillmentRequest) (*provider.FulfillmentResult, error) {
	targetID := req.ProviderItemID
	if targetID == "" {
		targetID = strings.TrimPrefix(req.ProductSlug, "silvex_")
	}

	// Try resolving exact product ID from catalog if needed
	if rawProds, err := a.client.GetProducts(ctx); err == nil {
		for _, p := range rawProds {
			pSlug := fmt.Sprintf("silvex_%s", strings.ToLower(strings.ReplaceAll(p.Name, " ", "_")))
			if strings.EqualFold(pSlug, req.ProductSlug) || strings.EqualFold(fmt.Sprintf("%v", p.ID), targetID) {
				if p.ProductID != "" {
					targetID = p.ProductID
				} else if p.ID != nil {
					targetID = fmt.Sprintf("%v", p.ID)
				}
				break
			}
		}
	}

	ordResp, err := a.client.CreateOrder(ctx, targetID, req.Quantity)
	if err != nil {
		return &provider.FulfillmentResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, err
	}

	// Check delivery status
	if strings.EqualFold(ordResp.Status, "Completed") && ordResp.OK {
		items := NormalizeDeliveredItems(ordResp.Delivered)
		return &provider.FulfillmentResult{
			Success:          true,
			ProviderOrderID:  ordResp.OrderID,
			DeliveredItems:   items,
			WholesaleCostUSD: ordResp.Total,
		}, nil
	}

	// Manual/Pending item on Silvex side
	if strings.EqualFold(ordResp.Status, "Pending") {
		return &provider.FulfillmentResult{
			Success:          false,
			ProviderOrderID:  ordResp.OrderID,
			DeliveredItems:   []string{},
			ErrorMessage:     "PENDING_MANUAL_DELIVERY",
			WholesaleCostUSD: ordResp.Total,
		}, nil
	}

	return &provider.FulfillmentResult{
		Success:          false,
		ProviderOrderID:  ordResp.OrderID,
		ErrorMessage:     fmt.Sprintf("status: %s, error: %s", ordResp.Status, ordResp.Error),
		WholesaleCostUSD: ordResp.Total,
	}, fmt.Errorf("silvex order status %s: %s", ordResp.Status, ordResp.Error)
}

// CheckHealth queries the Silvex balance and connection.
func (a *Adapter) CheckHealth(ctx context.Context) (*provider.HealthResult, error) {
	return a.client.CheckConnection(ctx)
}
