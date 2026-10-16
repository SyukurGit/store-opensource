package provider

import (
	"context"
	"testing"
	"time"

	"storeku/internal/models"
)

type mockProvider struct {
	id   string
	name string
}

func (m *mockProvider) ID() string   { return m.id }
func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) FetchCatalog(ctx context.Context) ([]*models.Product, error) {
	return []*models.Product{
		{Slug: m.id + "_prod", Name: m.name + " Product", Source: m.id},
	}, nil
}
func (m *mockProvider) GetLiveCost(ctx context.Context, itemID string, qty int) (float64, error) {
	return 10.0, nil
}
func (m *mockProvider) Fulfill(ctx context.Context, req *FulfillmentRequest) (*FulfillmentResult, error) {
	return &FulfillmentResult{Success: true, ProviderOrderID: m.id + "-123"}, nil
}
func (m *mockProvider) CheckHealth(ctx context.Context) (*HealthResult, error) {
	return &HealthResult{Success: true, ProviderID: m.id}, nil
}

func TestProviderRegistry(t *testing.T) {
	reg := NewRegistry()
	p1 := &mockProvider{id: "PROV1", name: "Provider 1"}
	p2 := &mockProvider{id: "PROV2", name: "Provider 2"}

	reg.Register(p1)
	reg.Register(p2)

	if len(reg.List()) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(reg.List()))
	}

	found, ok := reg.Get("PROV1")
	if !ok || found.Name() != "Provider 1" {
		t.Fatalf("failed to retrieve PROV1")
	}

	ctx := context.Background()
	prods, errs := reg.FetchAggregatedCatalog(ctx, nil, 2*time.Second)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(prods) != 2 {
		t.Fatalf("expected 2 aggregated products, got %d", len(prods))
	}

	res, err := reg.Fulfill(ctx, "PROV1", &FulfillmentRequest{InvoiceNumber: "INV-1"})
	if err != nil || !res.Success || res.ProviderOrderID != "PROV1-123" {
		t.Fatalf("fulfillment failed: %v, res: %+v", err, res)
	}
}
