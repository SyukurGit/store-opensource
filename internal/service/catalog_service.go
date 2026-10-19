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

	cacheMu        sync.RWMutex
	cachedProds    []*models.Product
	lastSyncTime   time.Time
	refreshTrigger chan struct{}
	syncMu         sync.Mutex
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
		registry:       registry,
		pricing:        pricing,
		overrideRepo:   overrideRepo,
		settingRepo:    settingRepo,
		db:             db,
		refreshTrigger: make(chan struct{}, 1),
	}
}

// PrewarmCache loads products from the persistent SQLite database into memory
// at application startup, guaranteeing < 1ms response time for initial visitors.
func (s *CatalogService) PrewarmCache() {
	if dbProds, err := s.loadFromPersistentDB(); err == nil && len(dbProds) > 0 {
		s.cacheMu.Lock()
		s.cachedProds = dbProds
		s.cacheMu.Unlock()
		log.Printf("[CATALOG PREWARM] Loaded %d products from local persistent DB into memory", len(dbProds))
	}
}

// TriggerBackgroundSync requests an asynchronous catalog refresh in the background without blocking.
func (s *CatalogService) TriggerBackgroundSync() {
	select {
	case s.refreshTrigger <- struct{}{}:
	default:
	}
}

// StartBackgroundWorker runs a continuous background worker that periodically
// syncs the catalog from remote providers without ever blocking public API requests.
func (s *CatalogService) StartBackgroundWorker(ctx context.Context, interval time.Duration) {
	log.Printf("[CATALOG WORKER] Started background catalog sync worker (interval: %v)", interval)

	// Initial sync in background
	_, _ = s.syncCatalog(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[CATALOG WORKER] Stopped background catalog sync worker")
			return
		case <-ticker.C:
			_, _ = s.syncCatalog(ctx)
		case <-s.refreshTrigger:
			log.Printf("[CATALOG WORKER] Handling on-demand refresh trigger")
			_, _ = s.syncCatalog(ctx)
		}
	}
}

// InvalidateCache flushes the in-memory aggregated catalog cache and triggers a background sync.
func (s *CatalogService) InvalidateCache() {
	s.cacheMu.Lock()
	s.cachedProds = nil
	s.lastSyncTime = time.Time{}
	s.cacheMu.Unlock()
	s.TriggerBackgroundSync()
}

// UpdateDisplayStatsMemory atomically updates a product's display order and sales count in memory immediately (0ms latency).
func (s *CatalogService) UpdateDisplayStatsMemory(slug string, displayOrder int, salesCount int) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for _, p := range s.cachedProds {
		if strings.EqualFold(p.Slug, slug) {
			p.DisplayOrder = displayOrder
			p.SalesCount = salesCount
			break
		}
	}
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

		// Set default fallback sales count if uninitialized (< 0)
		if p.SalesCount < 0 {
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
			if ov.SalesCount >= 0 {
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
			// ponytail: retain raw.DisplayOrder if already set by local product or upstream
		}
		p.IsActive = true

		// Preserve raw wholesale cost in BasePriceUSD
		p.BasePriceUSD = p.PriceUSD

		// Calculate exact retail USD and reference strikethrough USD based on active rate
		usdRate := 18000.0
		if s.pricing != nil {
			usdRate = s.pricing.GetUSDRate()
		}
		if usdRate <= 0 {
			usdRate = 18000.0
		}
		p.PriceUSD = math.Round((float64(p.PriceIDR)/usdRate)*100.0) / 100.0
		p.ReferencePriceUSD = math.Round((float64(p.ReferencePriceIDR)/usdRate)*100.0) / 100.0

		// Apply multilingual translation if requested
		if lang == "en" {
			ApplyLanguageToProduct(&p, "en")
		}

		publicProducts = append(publicProducts, &p)
	}

	// Sort: positive DisplayOrder (> 0) ascending first (1, 2, 3...), followed by default (0),
	// then SalesCount DESC, then Name ASC
	sort.SliceStable(publicProducts, func(i, j int) bool {
		aOrder, bOrder := publicProducts[i].DisplayOrder, publicProducts[j].DisplayOrder
		if aOrder > 0 && bOrder > 0 {
			if aOrder != bOrder {
				return aOrder < bOrder
			}
		} else if aOrder > 0 && bOrder == 0 {
			return true
		} else if aOrder == 0 && bOrder > 0 {
			return false
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

// fetchAggregatedRaw returns the aggregated products immediately from memory or DB (SWR pattern).
func (s *CatalogService) fetchAggregatedRaw(ctx context.Context) ([]*models.Product, error) {
	// 1. Fast Path: In-memory cache (< 0.1ms)
	s.cacheMu.RLock()
	if len(s.cachedProds) > 0 {
		cached := s.cachedProds
		needRefresh := time.Since(s.lastSyncTime) > 2*time.Minute
		s.cacheMu.RUnlock()

		// If cache is older than 2 minutes, trigger background refresh non-blockingly
		if needRefresh {
			s.TriggerBackgroundSync()
		}
		return cached, nil
	}
	s.cacheMu.RUnlock()

	// 2. Second Path: SQLite persistent cache (< 1ms)
	if dbProds, err := s.loadFromPersistentDB(); err == nil && len(dbProds) > 0 {
		s.cacheMu.Lock()
		s.cachedProds = dbProds
		s.cacheMu.Unlock()
		s.TriggerBackgroundSync()
		log.Printf("[CATALOG SWR] Serving %d products from SQLite persistent cache", len(dbProds))
		return dbProds, nil
	}

	// 3. Cold Start Fallback: Only if both memory & SQLite are completely empty
	log.Printf("[CATALOG SWR] Initial cold-start: querying providers directly...")
	return s.syncCatalog(ctx)
}

// syncCatalog queries upstream providers, updates the persistent SQLite DB,
// and updates the in-memory cache. Protected by syncMu so only 1 sync runs at a time.
func (s *CatalogService) syncCatalog(ctx context.Context) ([]*models.Product, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

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
				continue
			}
			if p.ID() == "SILVEX" && !silvexEnabled {
				continue
			}
			activeProviders = append(activeProviders, p.ID())
		}
	}

	start := time.Now()
	var prods []*models.Product
	if s.registry != nil {
		pList, errMap := s.registry.FetchAggregatedCatalog(ctx, activeProviders, 4000*time.Millisecond)
		if len(errMap) > 0 {
			log.Printf("[CATALOG SYNC] Upstream provider sync notice: %v", errMap)
		}
		prods = pList
	}

	if len(prods) > 0 {
		s.saveToPersistentDB(prods)
		s.cacheMu.Lock()
		s.cachedProds = prods
		s.lastSyncTime = time.Now()
		s.cacheMu.Unlock()
		log.Printf("[CATALOG SYNC] Complete in %v. Updated %d products in memory & SQLite", time.Since(start), len(prods))
		return prods, nil
	}

	// Fallback to in-memory cached if upstream failed
	s.cacheMu.RLock()
	if len(s.cachedProds) > 0 {
		cached := s.cachedProds
		s.cacheMu.RUnlock()
		return cached, nil
	}
	s.cacheMu.RUnlock()

	// Fallback to DB
	if dbProds, err := s.loadFromPersistentDB(); err == nil && len(dbProds) > 0 {
		s.cacheMu.Lock()
		s.cachedProds = dbProds
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

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[CATALOG DB ERROR] Failed to begin transaction for cached_products: %v", err)
		return
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		_ = tx.Rollback()
		log.Printf("[CATALOG DB ERROR] Failed to prepare statement for cached_products: %v", err)
		return
	}
	defer stmt.Close()

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
		_, _ = stmt.Exec(p.Slug, rawIDStr, p.Name, p.Type, p.Logo, p.Region, p.Stock, stockCappedInt, p.Warranty, p.Description, p.PriceUSD, string(tiersBytes))
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[CATALOG DB ERROR] Failed to commit batch cached_products: %v", err)
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

	// 1. Scan raw rows into in-memory slice first
	type rawCachedItem struct {
		slug, rawID, name, prodType, logo, region, warranty, desc, tiersJSON string
		stock, stockCapped                                                  int
		priceUSD                                                            float64
	}
	var rawList []rawCachedItem

	for rows.Next() {
		var item rawCachedItem
		if err := rows.Scan(&item.slug, &item.rawID, &item.name, &item.prodType, &item.logo, &item.region,
			&item.stock, &item.stockCapped, &item.warranty, &item.desc, &item.priceUSD, &item.tiersJSON); err != nil {
			continue
		}
		rawList = append(rawList, item)
	}
	// CRITICAL: Close rows cursor immediately to release SQLite connection back to pool!
	_ = rows.Close()

	if len(rawList) == 0 {
		return nil, nil
	}

	// 2. Fetch pricing snapshot once (prevents 100+ individual DB setting queries)
	var snap PricingSnapshot
	if s.pricing != nil {
		snap = s.pricing.GetSnapshot()
	}

	// 3. Transform products purely in-memory (0 database queries during loop)
	var products []*models.Product
	for _, item := range rawList {
		var tiers []models.ProductTier
		_ = json.Unmarshal([]byte(item.tiersJSON), &tiers)

		baseUSD := item.priceUSD
		if len(tiers) > 0 && tiers[0].PriceUSD > 0 {
			baseUSD = tiers[0].PriceUSD
		}
		var sellIDR, modalIDR int64
		if s.pricing != nil {
			sellIDR = s.pricing.ConvertUSDToIDRWithSnapshot(baseUSD, snap)
			modalIDR = int64(math.Round(baseUSD * snap.Rate))
		}
		products = append(products, &models.Product{
			RawID:             item.rawID,
			Slug:              item.slug,
			Name:              item.name,
			Type:              item.prodType,
			Logo:              item.logo,
			Region:            item.region,
			Stock:             item.stock,
			StockCapped:       item.stockCapped == 1,
			Warranty:          item.warranty,
			Description:       item.desc,
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
