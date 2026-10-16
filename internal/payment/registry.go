package payment

import (
	"fmt"
	"strings"
	"sync"
)

// Registry manages all registered payment gateways
type Registry struct {
	mu        sync.RWMutex
	gateways  map[string]Gateway
	defaultGW string
}

// NewRegistry creates a new empty payment gateway registry
func NewRegistry() *Registry {
	return &Registry{
		gateways: make(map[string]Gateway),
	}
}

// Register registers a payment gateway into the registry
func (r *Registry) Register(g Gateway) {
	if g == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	id := strings.ToUpper(strings.TrimSpace(g.ID()))
	r.gateways[id] = g

	// If no default is set yet, the first registered gateway becomes the default
	if r.defaultGW == "" {
		r.defaultGW = id
	}
}

// RegisterAlias registers an alternative name that points to an existing gateway ID
func (r *Registry) RegisterAlias(alias, targetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cleanTarget := strings.ToUpper(strings.TrimSpace(targetID))
	g, ok := r.gateways[cleanTarget]
	if !ok {
		return fmt.Errorf("target gateway '%s' not registered", cleanTarget)
	}
	cleanAlias := strings.ToUpper(strings.TrimSpace(alias))
	r.gateways[cleanAlias] = g
	return nil
}

// SetDefault sets the default payment gateway ID
func (r *Registry) SetDefault(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cleanID := strings.ToUpper(strings.TrimSpace(id))
	if _, ok := r.gateways[cleanID]; !ok {
		return fmt.Errorf("gateway '%s' is not registered", cleanID)
	}
	r.defaultGW = cleanID
	return nil
}

// Get retrieves a gateway by its ID
func (r *Registry) Get(id string) (Gateway, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cleanID := strings.ToUpper(strings.TrimSpace(id))
	g, ok := r.gateways[cleanID]
	return g, ok
}

// Default returns the default payment gateway
func (r *Registry) Default() (Gateway, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultGW == "" {
		return nil, fmt.Errorf("tidak ada payment gateway yang terdaftar")
	}
	g, ok := r.gateways[r.defaultGW]
	if !ok {
		return nil, fmt.Errorf("gateway default '%s' tidak ditemukan", r.defaultGW)
	}
	return g, nil
}

// GetByMethod finds a gateway that supports the given method, or falls back to default
func (r *Registry) GetByMethod(method Method) (Gateway, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.gateways {
		for _, m := range g.SupportedMethods() {
			if m == method {
				return g, nil
			}
		}
	}

	return r.Default()
}

// List returns a list of all registered gateways
func (r *Registry) List() []Gateway {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []Gateway
	for _, g := range r.gateways {
		list = append(list, g)
	}
	return list
}
