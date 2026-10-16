package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/provider"
	"storeku/internal/repository"
)

// CatalogService manages multi-provider catalog aggregation, caching,
// price customization (overrides), surge protection, and language translation.
type CatalogService struct {
	registry     *provider.Registry
	pricing      *PricingService
	overrideRepo *repository.ProductOverrideRepository
	settingRepo  *repository.SettingRepository
	db           *database.DB

	cacheMu      sync.RWMutex
	cachedProds  []*models.Product
	cacheExpires time.Time
}

// NewCatalogService creates a new CatalogService.
func NewCatalogService(
	registry *provider.Registry,
	pricing *PricingService,
	overrideRepo *repository.ProductOverrideRepository,
	settingRepo *repository.SettingRepository,
	db *database.DB,
) *CatalogService {
	return &CatalogService{
		registry:     registry,
		pricing:      pricing,
		overrideRepo: overrideRepo,
		settingRepo:  settingRepo,
		db:           db,
	}
}

// InvalidateCache flushes the in-memory aggregated catalog cache.
func (s *CatalogService) InvalidateCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cachedProds = nil
	s.cacheExpires = time.Time{}
}

// GetPublicProducts retrieves the aggregated, active, customized product catalog.
func (s *CatalogService) GetPublicProducts(ctx context.Context, lang string) ([]*models.Product, error) {
	// 1. Fetch raw aggregated products (from cache or providers)
	rawProds, err := s.fetchAggregatedRaw(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Fetch admin overrides
	overrides := make(map[string]*models.ProductOverride)
	if s.overrideRepo != nil {
		if ovs, err := s.overrideRepo.GetAll(); err == nil {
			overrides = ovs
		}
	}

	maxSurgeDisablePct := 50.0
	if s.pricing != nil {
		maxSurgeDisablePct = s.pricing.GetMaxSurgeDisablePercent()
	}

	var publicProducts []*models.Product
	for _, raw := range rawProds {
		p := *raw // shallow copy

		// Enforce minimum sales count
		if p.SalesCount <= 0 {
			p.SalesCount = 10
		}

		// Calculate default pricing if USD-denominated
		defaultEnvPrice := p.PriceIDR
		if p.PriceUSD > 0 && s.pricing != nil {
			defaultEnvPrice = s.pricing.ConvertUSDToIDR(p.PriceUSD)
		}
		minSafePrice := defaultEnvPrice

		// Apply Overrides if present
		if ov, hasOv := overrides[p.Slug]; hasOv {
			if !ov.IsActive {
				continue
			}

			p.IsForceOutOfStock = ov.IsForceOutOfStock
			if ov.IsForceOutOfStock {
				p.Stock = 0
			} else if ov.CustomStock >= 0 {
				p.Stock = ov.CustomStock
			}

			if ov.CustomName != "" {
				p.Name = ov.CustomName
			}
			if ov.CustomType != "" {
				p.Type = ov.CustomType
			}
			if ov.CustomRegion != "" {
				p.Region = ov.CustomRegion
			}
			if ov.CustomWarranty != "" {
				p.Warranty = ov.CustomWarranty
			}
			if ov.CustomLogo != "" {
				p.Logo = ov.CustomLogo
			}
			if ov.CustomNameEN != "" {
				p.NameEN = ov.CustomNameEN
			}
			if ov.CustomTypeEN != "" {
				p.TypeEN = ov.CustomTypeEN
			}
			if ov.CustomRegionEN != "" {
				p.RegionEN = ov.CustomRegionEN
			}
			if ov.CustomWarrantyEN != "" {
				p.WarrantyEN = ov.CustomWarrantyEN
			}
			if ov.CustomDescriptionEN != "" {
				p.DescriptionEN = ov.CustomDescriptionEN
			}
			if ov.CustomTermsEN != "" {
				p.TermsEN = ov.CustomTermsEN
			}

			// Baseline modal tracking & surge evaluation
			baselineModal := ov.BaselineModalIDR
			if baselineModal <= 0 {
				baselineModal = p.ModalIDR
			}

			deltaModal := p.ModalIDR - baselineModal
			var surgePct float64
			if baselineModal > 0 && deltaModal > 0 {
				surgePct = (float64(deltaModal) / float64(baselineModal)) * 100.0
			}

			// If surge is too extreme, AUTO-DISABLE product from public catalog
			if surgePct > maxSurgeDisablePct {
				continue
			}

			refPrice := defaultEnvPrice
			if ov.CustomPriceIDR > 0 {
				refPrice = ov.CustomPriceIDR
				if deltaModal > 0 {
					refPrice += deltaModal // Pass-through delta
				}
			}
			if refPrice < minSafePrice {
				refPrice = minSafePrice
			}
			p.ReferencePriceIDR = refPrice

			// Promo Price with Dynamic Pass-Through
			if ov.PromoPriceIDR > 0 {
				promoCandidate := ov.PromoPriceIDR
				if deltaModal > 0 {
					promoCandidate += deltaModal
				}

				minModalFloor := p.ModalIDR
				if deltaModal > 0 {
					minModalFloor += deltaModal
				}
				if promoCandidate < minModalFloor {
					promoCandidate = minModalFloor
				}

				if promoCandidate >= refPrice || (ov.CustomPriceIDR > 0 && deltaModal > 0 && promoCandidate >= ov.CustomPriceIDR) {
					p.PromoPriceIDR = 0
					p.PriceIDR = refPrice
					p.HasPromo = false
					p.DiscountPercent = 0
				} else {
					p.PromoPriceIDR = promoCandidate
					p.PriceIDR = promoCandidate
					p.HasPromo = true
					p.DiscountPercent = int(math.Round(float64(refPrice-promoCandidate) / float64(refPrice) * 100))
				}
			} else {
				p.PromoPriceIDR = 0
				p.PriceIDR = refPrice
				p.HasPromo = false
				p.DiscountPercent = 0
			}

			if ov.CustomDescription != "" {
				p.Description = ov.CustomDescription
			}
			if ov.CustomTerms != "" {
				p.Terms = ov.CustomTerms
			} else {
				p.Terms = "Membeli berarti setuju"
			}
			p.IsCustomized = true
			p.BaselineModalIDR = baselineModal
			p.SurgePercent = surgePct
			p.DisplayOrder = ov.DisplayOrder
			if ov.SalesCount > 0 {
				p.SalesCount = ov.SalesCount
			}

			// Clear conflicting wholesale tiers for customized products
			p.Tiers = nil
		} else {
			p.Terms = "Membeli berarti setuju"
			p.ReferencePriceIDR = p.PriceIDR
			p.PromoPriceIDR = 0
			p.HasPromo = false
			p.DiscountPercent = 0
			p.DisplayOrder = 0
		}
		p.IsActive = true

		// Apply multilingual translation if requested
		if lang == "en" {
			ApplyLanguageToProduct(&p, "en")
		}

		publicProducts = append(publicProducts, &p)
	}

	// Sort: DisplayOrder ASC, SalesCount DESC, Name ASC
	sort.SliceStable(publicProducts, func(i, j int) bool {
		if publicProducts[i].DisplayOrder != publicProducts[j].DisplayOrder {
			return publicProducts[i].DisplayOrder < publicProducts[j].DisplayOrder
		}
		if publicProducts[i].SalesCount != publicProducts[j].SalesCount {
			return publicProducts[i].SalesCount > publicProducts[j].SalesCount
		}
		return publicProducts[i].Name < publicProducts[j].Name
	})

	return publicProducts, nil
}

// GetPublicProductBySlug finds an active public product by slug.
func (s *CatalogService) GetPublicProductBySlug(ctx context.Context, slug, lang string) (*models.Product, error) {
	products, err := s.GetPublicProducts(ctx, lang)
	if err != nil {
		return nil, err
	}

	for _, p := range products {
		if strings.EqualFold(p.Slug, slug) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("produk '%s' tidak ditemukan atau sedang tidak aktif", slug)
}

// fetchAggregatedRaw fetches from in-memory cache or active providers.
func (s *CatalogService) fetchAggregatedRaw(ctx context.Context) ([]*models.Product, error) {
	// 1. Check in-memory cache
	s.cacheMu.RLock()
	if len(s.cachedProds) > 0 && time.Now().Before(s.cacheExpires) {
		cached := s.cachedProds
		s.cacheMu.RUnlock()
		return cached, nil
	}
	s.cacheMu.RUnlock()

	// 2. Determine active providers
	dcEnabled := true
	silvexEnabled := true
	if s.settingRepo != nil {
		dcEnabled = s.settingRepo.Get("digitalcore_enabled", "true") != "false"
		silvexEnabled = s.settingRepo.Get("silvex_enabled", "true") != "false"
	}

	var activeProviders []string
	if s.registry != nil {
		for _, p := range s.registry.List() {
			if p.ID() == "DIGITALCORE" && !dcEnabled {
				continue // Skip if DigitalCore upstream sync turned OFF
			}
			if p.ID() == "SILVEX" && !silvexEnabled {
				continue // Skip if Silvex AI upstream sync turned OFF
			}
			activeProviders = append(activeProviders, p.ID())
		}
	}

	// 3. Query providers concurrently with a 3.5s timeout
	var prods []*models.Product
	if s.registry != nil {
		pList, errMap := s.registry.FetchAggregatedCatalog(ctx, activeProviders, 3500*time.Millisecond)
		if len(errMap) > 0 {
			log.Printf("[CATALOG] Upstream provider errors: %v", errMap)
		}
		prods = pList
	}

	// 4. If upstream returned products, save to persistent DB & memory cache
	if len(prods) > 0 {
		s.saveToPersistentDB(prods)
		s.cacheMu.Lock()
		s.cachedProds = prods
		s.cacheExpires = time.Now().Add(45 * time.Second)
		s.cacheMu.Unlock()
		return prods, nil
	}

	// 5. Fallback to in-memory cached if present
	s.cacheMu.RLock()
	if len(s.cachedProds) > 0 {
		cached := s.cachedProds
		s.cacheMu.RUnlock()
		log.Printf("[CATALOG FALLBACK] Serving last in-memory cached catalog (%d products)", len(cached))
		return cached, nil
	}
	s.cacheMu.RUnlock()

	// 6. Fallback to persistent SQLite table cached_products
	if dbProds, err := s.loadFromPersistentDB(); err == nil && len(dbProds) > 0 {
		log.Printf("[CATALOG FALLBACK] Serving persistent DB cached catalog (%d products)", len(dbProds))
		s.cacheMu.Lock()
		s.cachedProds = dbProds
		s.cacheExpires = time.Now().Add(45 * time.Second)
		s.cacheMu.Unlock()
		return dbProds, nil
	}

	return nil, fmt.Errorf("katalog produk kosong atau penyedia tidak dapat dijangkau")
}

func (s *CatalogService) saveToPersistentDB(prods []*models.Product) {
	if s.db == nil {
		return
	}
	query := `
		INSERT INTO cached_products (
			slug, raw_id, name, type, logo, region, stock, stock_capped, warranty, description, price_usd, tiers_json, last_fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(slug) DO UPDATE SET
			raw_id = excluded.raw_id,
			name = excluded.name,
			type = excluded.type,
			logo = excluded.logo,
			region = excluded.region,
			stock = excluded.stock,
			stock_capped = excluded.stock_capped,
			warranty = excluded.warranty,
			description = excluded.description,
			price_usd = excluded.price_usd,
			tiers_json = excluded.tiers_json,
			last_fetched_at = CURRENT_TIMESTAMP
	`
	for _, p := range prods {
		if p.Source == "LOCAL" {
			continue // Local products are stored in local_products table
		}
		rawIDStr := fmt.Sprintf("%v", p.RawID)
		stockCappedInt := 0
		if p.StockCapped {
			stockCappedInt = 1
		}
		tiersBytes, _ := json.Marshal(p.Tiers)
		_, _ = s.db.Exec(query, p.Slug, rawIDStr, p.Name, p.Type, p.Logo, p.Region, p.Stock, stockCappedInt, p.Warranty, p.Description, p.PriceUSD, string(tiersBytes))
	}
}

func (s *CatalogService) loadFromPersistentDB() ([]*models.Product, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	query := `
		SELECT slug, raw_id, name, type, logo, region, stock, stock_capped, warranty, description, price_usd, tiers_json
		FROM cached_products
		ORDER BY slug ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		var slug, rawID, name, prodType, logo, region, warranty, desc, tiersJSON string
		var stock, stockCapped int
		var priceUSD float64
		if err := rows.Scan(&slug, &rawID, &name, &prodType, &logo, &region, &stock, &stockCapped, &warranty, &desc, &priceUSD, &tiersJSON); err != nil {
			continue
		}
		var tiers []models.ProductTier
		_ = json.Unmarshal([]byte(tiersJSON), &tiers)

		baseUSD := priceUSD
		if len(tiers) > 0 && tiers[0].PriceUSD > 0 {
			baseUSD = tiers[0].PriceUSD
		}
		var sellIDR, modalIDR int64
		if s.pricing != nil {
			sellIDR = s.pricing.ConvertUSDToIDR(baseUSD)
			modalIDR = int64(math.Round(baseUSD * s.pricing.GetUSDRate()))
		}
		products = append(products, &models.Product{
			RawID:             rawID,
			Slug:              slug,
			Name:              name,
			Type:              prodType,
			Logo:              logo,
			Region:            region,
			Stock:             stock,
			StockCapped:       stockCapped == 1,
			Warranty:          warranty,
			Description:       desc,
			Terms:             "Membeli berarti setuju",
			PriceUSD:          baseUSD,
			ModalIDR:          modalIDR,
			PriceIDR:          sellIDR,
			ReferencePriceIDR: sellIDR,
			Tiers:             tiers,
			IsActive:          true,
			Source:            "DIGITALCORE",
		})
	}
	return products, nil
}

// ApplyLanguageToProduct swaps ID fields with EN fields if lang == "en".
func ApplyLanguageToProduct(prod *models.Product, lang string) {
	if lang != "en" {
		return
	}
	if prod.NameEN != "" {
		prod.Name = prod.NameEN
	}
	if prod.TypeEN != "" {
		prod.Type = prod.TypeEN
	}
	if prod.RegionEN != "" {
		prod.Region = prod.RegionEN
	}
	if prod.WarrantyEN != "" {
		prod.Warranty = prod.WarrantyEN
	}
	if prod.DescriptionEN != "" {
		prod.Description = prod.DescriptionEN
	}
	if prod.TermsEN != "" {
		prod.Terms = prod.TermsEN
	}
}
