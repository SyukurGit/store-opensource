package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"storeku/internal/models"
)

// Registry manages all registered product providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new empty Provider Registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
}

// Get returns the provider by ID.
func (r *Registry) Get(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

// List returns all registered providers.
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		list = append(list, p)
	}
	return list
}

// FetchAggregatedCatalog queries all specified providers concurrently within a timeout
// and merges their canonical products into a unified slice.
func (r *Registry) FetchAggregatedCatalog(ctx context.Context, providerIDs []string, timeout time.Duration) ([]*models.Product, map[string]error) {
	r.mu.RLock()
	var targets []Provider
	if len(providerIDs) == 0 {
		for _, p := range r.providers {
			targets = append(targets, p)
		}
	} else {
		for _, id := range providerIDs {
			if p, ok := r.providers[id]; ok {
				targets = append(targets, p)
			}
		}
	}
	r.mu.RUnlock()

	type providerRes struct {
		providerID string
		prods      []*models.Product
		err        error
	}

	resChan := make(chan providerRes, len(targets))
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, p := range targets {
		wg.Add(1)
		go func(prov Provider) {
			defer wg.Done()
			prods, err := prov.FetchCatalog(ctxTimeout)
			resChan <- providerRes{
				providerID: prov.ID(),
				prods:      prods,
				err:        err,
			}
		}(p)
	}

	// Wait in background and close channel
	go func() {
		wg.Wait()
		close(resChan)
	}()

	var allProds []*models.Product
	errMap := make(map[string]error)

	for res := range resChan {
		if res.err != nil {
			errMap[res.providerID] = res.err
		} else if len(res.prods) > 0 {
			allProds = append(allProds, res.prods...)
		}
	}

	return allProds, errMap
}

// GetLiveCost finds the provider for the product and retrieves its live wholesale cost.
func (r *Registry) GetLiveCost(ctx context.Context, providerID, itemID string, qty int) (float64, error) {
	p, ok := r.Get(providerID)
	if !ok {
		return 0, fmt.Errorf("provider '%s' tidak terdaftar", providerID)
	}
	return p.GetLiveCost(ctx, itemID, qty)
}

// Fulfill routes fulfillment to the appropriate provider.
func (r *Registry) Fulfill(ctx context.Context, providerID string, req *FulfillmentRequest) (*FulfillmentResult, error) {
	p, ok := r.Get(providerID)
	if !ok {
		return nil, fmt.Errorf("provider '%s' tidak terdaftar", providerID)
	}
	return p.Fulfill(ctx, req)
}
