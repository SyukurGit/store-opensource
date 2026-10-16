package provider

import (
	"context"

	"storeku/internal/models"
)

// Provider defines the standard contract that every product supplier (local database,
// DigitalCore, or future providers such as Digiflazz, VIP Reseller, Tokovoucher, etc.)
// must satisfy to integrate seamlessly into Digimest.
type Provider interface {
	// ID returns the unique system identifier for this provider (e.g. "LOCAL", "DIGITALCORE")
	ID() string

	// Name returns human-readable display name for this provider
	Name() string

	// FetchCatalog fetches all available products from this provider and normalizes
	// them into the canonical StoreKu models.Product baseline.
	FetchCatalog(ctx context.Context) ([]*models.Product, error)

	// GetLiveCost queries the provider for real-time wholesale cost for a product and quantity.
	// Returns cost in USD (if USD-denominated) or IDR (if IDR-denominated).
	GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error)

	// Fulfill executes the purchase/delivery of the digital item(s).
	Fulfill(ctx context.Context, req *FulfillmentRequest) (*FulfillmentResult, error)

	// CheckHealth verifies connection status, credentials, and retrieves balance.
	CheckHealth(ctx context.Context) (*HealthResult, error)
}
