package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/repository"
)

type DigitalCoreClient struct {
	cfg          *config.Config
	httpClient   *http.Client
	pricing      *PricingService
	overrideRepo *repository.ProductOverrideRepository
	db           *database.DB
	cacheMu      sync.RWMutex
	cachedProds  []*models.Product
	cacheExpires time.Time
}

func NewDigitalCoreClient(cfg *config.Config, pricing *PricingService, overrideRepo *repository.ProductOverrideRepository) *DigitalCoreClient {
	return &DigitalCoreClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		pricing:      pricing,
		overrideRepo: overrideRepo,
	}
}

// SetDB attaches the database connection for persistent catalog caching
func (c *DigitalCoreClient) SetDB(db *database.DB) {
	c.db = db
}

// DigitalCoreRawProduct matches the structure returned by DigitalCore /api/public/products
type DigitalCoreRawProduct struct {
	ID          interface{} `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Logo        string      `json:"logo"`
	Region      string      `json:"region"`
	Status      string      `json:"status"`
	Stock       int         `json:"stock"`
	StockCapped bool        `json:"stockCapped"`
	Warranty    string      `json:"warranty"`
	Desc        string      `json:"desc"`
	PriceFrom   float64     `json:"priceFrom"`
	Tiers       []struct {
		MinQty int     `json:"minQty"`
		MaxQty *int    `json:"maxQty"`
		Price  float64 `json:"price"`
	} `json:"tiers"`
}

// DigitalCoreBuyResponse matches the response of POST /api/user/buy
type DigitalCoreBuyResponse struct {
	Success     bool     `json:"success"`
	OrderID     string   `json:"order_id"`
	ViewURL     string   `json:"view_url"`
	TotalAmount float64  `json:"total_amount"`
	Quantity    int      `json:"quantity"`
	Items       []string `json:"items"`
	Msg         string   `json:"msg"`
}

// DigitalCoreMeResponse matches GET /api/user/me
type DigitalCoreMeResponse struct {
	Balance     float64     `json:"balance"`
	TotalOrders interface{} `json:"total_orders"`
}

func toSlug(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "_")
}

// fallbackCachedOrError returns last cached products if available, or the error if empty
func (c *DigitalCoreClient) fallbackCachedOrError(err error) ([]*models.Product, error) {
	c.cacheMu.RLock()
	if len(c.cachedProds) > 0 {
		cached := c.cachedProds
		c.cacheMu.RUnlock()
		log.Printf("[DIGITALCORE FALLBACK] Upstream micro-outage: %v. Serving last cached catalog (%d products).", err, len(cached))
		return cached, nil
	}
	c.cacheMu.RUnlock()

	// Fallback to SQLite DB table cached_products
	if c.db != nil {
		if dbProds, dbErr := c.loadProductsFromDB(); dbErr == nil && len(dbProds) > 0 {
			log.Printf("[DIGITALCORE FALLBACK] Upstream micro-outage: %v. Serving persistent DB cached catalog (%d products).", err, len(dbProds))
			c.cacheMu.Lock()
			c.cachedProds = dbProds
			c.cacheExpires = time.Now().Add(45 * time.Second)
			c.cacheMu.Unlock()
			return dbProds, nil
		}
	}

	return nil, err
}

func (c *DigitalCoreClient) saveProductsToDB(rawProds []DigitalCoreRawProduct) {
	if c.db == nil {
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
	for _, p := range rawProds {
		slug := toSlug(p.Name)
		tiersBytes, _ := json.Marshal(p.Tiers)
		rawIDStr := fmt.Sprintf("%v", p.ID)
		stockCappedInt := 0
		if p.StockCapped {
			stockCappedInt = 1
		}
		var logoURL string
		if p.Logo != "" {
			if strings.HasPrefix(p.Logo, "http") {
				logoURL = p.Logo
			} else {
				logoURL = fmt.Sprintf("%s%s", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"), p.Logo)
			}
		}
		_, _ = c.db.Exec(query, slug, rawIDStr, p.Name, p.Type, logoURL, p.Region, p.Stock, stockCappedInt, p.Warranty, p.Desc, p.PriceFrom, string(tiersBytes))
	}
}

func (c *DigitalCoreClient) loadProductsFromDB() ([]*models.Product, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	query := `
		SELECT slug, raw_id, name, type, logo, region, stock, stock_capped, warranty, description, price_usd, tiers_json
		FROM cached_products
		ORDER BY slug ASC
	`
	rows, err := c.db.Query(query)
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
		var rawTiers []struct {
			MinQty int     `json:"minQty"`
			MaxQty *int    `json:"maxQty"`
			Price  float64 `json:"price"`
		}
		_ = json.Unmarshal([]byte(tiersJSON), &rawTiers)
		var tiers []models.ProductTier
		for _, t := range rawTiers {
			var priceIDR int64
			if c.pricing != nil {
				priceIDR = c.pricing.ConvertUSDToIDR(t.Price)
			}
			tiers = append(tiers, models.ProductTier{
				MinQty:   t.MinQty,
				MaxQty:   t.MaxQty,
				PriceUSD: t.Price,
				PriceIDR: priceIDR,
			})
		}
		baseUSD := priceUSD
		if len(tiers) > 0 && tiers[0].PriceUSD > 0 {
			baseUSD = tiers[0].PriceUSD
		}
		var sellIDR, modalIDR int64
		if c.pricing != nil {
			sellIDR = c.pricing.ConvertUSDToIDR(baseUSD)
			modalIDR = int64(math.Round(baseUSD * c.pricing.GetUSDRate()))
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
			IsCustomized:      false,
		})
	}
	return products, nil
}

// FetchRawProducts fetches all active products directly from DigitalCore and computes baseline IDR
func (c *DigitalCoreClient) FetchRawProducts() ([]*models.Product, error) {
	// 1. Check in-memory cache first (45s TTL)
	c.cacheMu.RLock()
	if len(c.cachedProds) > 0 && time.Now().Before(c.cacheExpires) {
		cached := c.cachedProds
		c.cacheMu.RUnlock()
		return cached, nil
	}
	c.cacheMu.RUnlock()

	// 2. Fetch fresh products from DigitalCore
	url := fmt.Sprintf("%s/api/public/products", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return c.fallbackCachedOrError(fmt.Errorf("failed to create request: %w", err))
	}

	if c.cfg.DigitalCoreAPIKey != "" {
		req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.fallbackCachedOrError(fmt.Errorf("failed to execute request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.fallbackCachedOrError(fmt.Errorf("digitalcore returned status %d: %s", resp.StatusCode, string(body)))
	}

	var rawProds []DigitalCoreRawProduct
	if err := json.NewDecoder(resp.Body).Decode(&rawProds); err != nil {
		return c.fallbackCachedOrError(fmt.Errorf("failed to decode products: %w", err))
	}

	var products []*models.Product
	for _, p := range rawProds {
		if strings.ToLower(p.Status) != "active" {
			continue
		}

		slug := toSlug(p.Name)
		var logoURL string
		if p.Logo != "" {
			if strings.HasPrefix(p.Logo, "http") {
				logoURL = p.Logo
			} else {
				logoURL = fmt.Sprintf("%s%s", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"), p.Logo)
			}
		}

		// Calculate tier pricing in IDR
		var tiers []models.ProductTier
		for _, t := range p.Tiers {
			tiers = append(tiers, models.ProductTier{
				MinQty:   t.MinQty,
				MaxQty:   t.MaxQty,
				PriceUSD: t.Price,
				PriceIDR: c.pricing.ConvertUSDToIDR(t.Price),
			})
		}

		// If tiers exist, calculate base price from tier 1 or priceFrom
		baseUSD := p.PriceFrom
		if len(tiers) > 0 && tiers[0].PriceUSD > 0 {
			baseUSD = tiers[0].PriceUSD
		}
		sellIDR := c.pricing.ConvertUSDToIDR(baseUSD)
		modalIDR := int64(math.Round(baseUSD * c.pricing.GetUSDRate()))

		products = append(products, &models.Product{
			RawID:             p.ID,
			Slug:              slug,
			Name:              p.Name,
			Type:              p.Type,
			Logo:              logoURL,
			Region:            p.Region,
			Stock:             p.Stock,
			StockCapped:       p.StockCapped,
			Warranty:          p.Warranty,
			Description:       p.Desc,
			Terms:             "Membeli berarti setuju",
			PriceUSD:          baseUSD,
			ModalIDR:          modalIDR,
			PriceIDR:          sellIDR,
			ReferencePriceIDR: sellIDR,
			PromoPriceIDR:     0,
			HasPromo:          false,
			DiscountPercent:   0,
			Tiers:             tiers,
			IsActive:          true,
			IsCustomized:      false,
		})
	}

	// 3. Save into cache with 45s TTL
	c.cacheMu.Lock()
	c.cachedProds = products
	c.cacheExpires = time.Now().Add(45 * time.Second)
	c.cacheMu.Unlock()

	// 4. Persist into SQLite DB table cached_products
	c.saveProductsToDB(rawProds)

	return products, nil
}

// FetchProducts returns only active products with overrides merged for the public store
func (c *DigitalCoreClient) FetchProducts() ([]*models.Product, error) {
	rawProducts, err := c.FetchRawProducts()
	if err != nil {
		return nil, err
	}

	overrides := make(map[string]*models.ProductOverride)
	if c.overrideRepo != nil {
		if ovs, err := c.overrideRepo.GetAll(); err == nil {
			overrides = ovs
		}
	}

	maxSurgeDisablePct := c.pricing.GetMaxSurgeDisablePercent()

	var publicProducts []*models.Product
	for _, rawP := range rawProducts {
		// Deep copy to prevent mutating the in-memory cached products
		p := *rawP
		if rawP.Tiers != nil {
			p.Tiers = make([]models.ProductTier, len(rawP.Tiers))
			copy(p.Tiers, rawP.Tiers)
		}

		// Calculate pristine default environment formula price
		defaultEnvPrice := c.pricing.ConvertUSDToIDR(rawP.PriceUSD)
		minSafePrice := defaultEnvPrice
		p.PriceIDR = defaultEnvPrice
		p.ReferencePriceIDR = defaultEnvPrice

		ov, hasOv := overrides[p.Slug]
		if hasOv {
			// If disabled by admin, do not display in public catalog
			if !ov.IsActive {
				continue
			}

			// Stock Override
			p.CustomStock = ov.CustomStock
			if ov.IsForceOutOfStock {
				p.IsForceOutOfStock = true
				p.Stock = 0
			} else if ov.CustomStock >= 0 {
				p.Stock = ov.CustomStock
			}

			if ov.CustomName != "" {
				p.Name = ov.CustomName
			}
			if ov.CustomType != "" {
				p.CustomType = ov.CustomType
				p.Type = ov.CustomType
			}
			if ov.CustomRegion != "" {
				p.CustomRegion = ov.CustomRegion
				p.Region = ov.CustomRegion
			}
			if ov.CustomWarranty != "" {
				p.CustomWarranty = ov.CustomWarranty
				p.Warranty = ov.CustomWarranty
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

			// If surge is too extreme, AUTO-DISABLE product from public catalog!
			if surgePct > maxSurgeDisablePct {
				continue
			}

			refPrice := defaultEnvPrice
			if ov.CustomPriceIDR > 0 {
				refPrice = ov.CustomPriceIDR
				if deltaModal > 0 {
					refPrice += deltaModal // Pass-through delta to reference price
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
					promoCandidate += deltaModal // Pass-through delta to promo price
				}

				// Golden Rule: Promo must not be lower than minSafePrice (Modal + margin)
				if promoCandidate < minSafePrice {
					promoCandidate = minSafePrice
				}

				// If promo price equals or exceeds reference price or original base price during surge, banner promo turns off!
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

			// Clear conflicting wholesale tiers for customized products so detail page & checkout match root catalog exactly
			p.Tiers = nil
		} else {
			p.Terms = "Membeli berarti setuju"
			p.ReferencePriceIDR = p.PriceIDR
			p.PromoPriceIDR = 0
			p.HasPromo = false
			p.DiscountPercent = 0
		}
		p.IsActive = true
		publicProducts = append(publicProducts, &p)
	}

	return publicProducts, nil
}

// GetPublicProductBySlug retrieves a single product by slug if active
func (c *DigitalCoreClient) GetPublicProductBySlug(slug string) (*models.Product, error) {
	products, err := c.FetchProducts()
	if err != nil {
		return nil, err
	}

	for _, p := range products {
		if p.Slug == slug {
			return p, nil
		}
	}

	return nil, fmt.Errorf("produk '%s' tidak ditemukan atau sedang tidak aktif", slug)
}

// FetchAdminProducts returns all upstream products merged with override configurations
func (c *DigitalCoreClient) FetchAdminProducts() ([]*models.AdminProductItem, error) {
	rawProducts, err := c.FetchRawProducts()
	if err != nil {
		return nil, err
	}

	overrides := make(map[string]*models.ProductOverride)
	if c.overrideRepo != nil {
		if ovs, err := c.overrideRepo.GetAll(); err == nil {
			overrides = ovs
		}
	}

	var adminItems []*models.AdminProductItem
	for _, p := range rawProducts {
		modalIDR := int64(math.Round(p.PriceUSD * c.pricing.GetUSDRate()))
		envPriceIDR := c.pricing.ConvertUSDToIDR(p.PriceUSD)

		item := &models.AdminProductItem{
			Slug:                 p.Slug,
			OriginalName:         p.Name,
			CustomName:           "",
			Name:                 p.Name,
			OriginalType:         p.Type,
			CustomType:           "",
			Type:                 p.Type,
			Logo:                 p.Logo,
			OriginalRegion:       p.Region,
			CustomRegion:         "",
			Region:               p.Region,
			OriginalStock:        p.Stock,
			CustomStock:          -1,
			Stock:                p.Stock,
			OriginalWarranty:     p.Warranty,
			CustomWarranty:       "",
			Warranty:             p.Warranty,
			OriginalPriceUSD:     p.PriceUSD,
			ModalIDR:             modalIDR,
			EnvPriceIDR:          envPriceIDR,
			CalculatedPriceIDR:   envPriceIDR,
			CustomPriceIDR:       0,
			ReferencePriceIDR:    envPriceIDR,
			PromoPriceIDR:        0,
			PriceIDR:             envPriceIDR,
			HasPromo:             false,
			DiscountPercent:      0,
			OriginalDescription:  p.Description,
			CustomDescription:    "",
			Description:          p.Description,
			CustomTerms:          "",
			Terms:                "Membeli berarti setuju",
			IsActive:             true,
			IsForceOutOfStock:    false,
			IsOriginalOutOfStock: (p.Stock <= 0),
			IsCustomized:         false,
			Tiers:                p.Tiers,
		}

		maxSurgeDisablePct := c.pricing.GetMaxSurgeDisablePercent()

		if ov, hasOv := overrides[p.Slug]; hasOv {
			item.IsActive = ov.IsActive
			item.IsForceOutOfStock = ov.IsForceOutOfStock
			item.IsCustomized = true
			item.CustomName = ov.CustomName
			if ov.CustomName != "" {
				item.Name = ov.CustomName
			}
			item.CustomType = ov.CustomType
			if ov.CustomType != "" {
				item.Type = ov.CustomType
			}
			item.CustomRegion = ov.CustomRegion
			if ov.CustomRegion != "" {
				item.Region = ov.CustomRegion
			}
			item.CustomWarranty = ov.CustomWarranty
			if ov.CustomWarranty != "" {
				item.Warranty = ov.CustomWarranty
			}
			item.CustomStock = ov.CustomStock
			if ov.IsForceOutOfStock {
				item.Stock = 0
			} else if ov.CustomStock >= 0 {
				item.Stock = ov.CustomStock
			}
			item.CustomPriceIDR = ov.CustomPriceIDR

			baselineModal := ov.BaselineModalIDR
			if baselineModal <= 0 {
				baselineModal = p.ModalIDR
			}
			item.BaselineModalIDR = baselineModal

			deltaModal := p.ModalIDR - baselineModal
			var surgePct float64
			if baselineModal > 0 && deltaModal > 0 {
				surgePct = (float64(deltaModal) / float64(baselineModal)) * 100.0
			}
			item.SurgePercent = surgePct

			if surgePct > maxSurgeDisablePct {
				item.IsAutoSuspended = true
			}

			// Minimum safe price ensuring default profit margin
			minSafePrice := envPriceIDR

			refPrice := envPriceIDR
			if ov.CustomPriceIDR > 0 {
				refPrice = ov.CustomPriceIDR
				if deltaModal > 0 {
					refPrice += deltaModal // Pass-through delta
				}
			}
			if refPrice < minSafePrice {
				refPrice = minSafePrice
			}
			item.ReferencePriceIDR = refPrice

			if ov.PromoPriceIDR > 0 {
				promoCandidate := ov.PromoPriceIDR
				if deltaModal > 0 {
					promoCandidate += deltaModal // Pass-through delta
				}

				if promoCandidate < minSafePrice {
					promoCandidate = minSafePrice
				}

				if promoCandidate >= refPrice || (ov.CustomPriceIDR > 0 && deltaModal > 0 && promoCandidate >= ov.CustomPriceIDR) {
					item.PromoPriceIDR = 0
					item.PriceIDR = refPrice
					item.HasPromo = false
					item.DiscountPercent = 0
				} else {
					item.PromoPriceIDR = promoCandidate
					item.PriceIDR = promoCandidate
					item.HasPromo = true
					item.DiscountPercent = int(math.Round(float64(refPrice-promoCandidate) / float64(refPrice) * 100))
				}
			} else {
				item.PromoPriceIDR = 0
				item.PriceIDR = refPrice
				item.HasPromo = false
				item.DiscountPercent = 0
			}

			item.CustomDescription = ov.CustomDescription
			if ov.CustomDescription != "" {
				item.Description = ov.CustomDescription
			}
			item.CustomTerms = ov.CustomTerms
			if ov.CustomTerms != "" {
				item.Terms = ov.CustomTerms
			}
		}

		adminItems = append(adminItems, item)
	}

	return adminItems, nil
}

// GetProductLiveCost queries DigitalCore directly for the live wholesale USD cost of a product for given quantity
func (c *DigitalCoreClient) GetProductLiveCost(slug string, qty int) (float64, error) {
	rawProducts, err := c.FetchRawProducts()
	if err != nil {
		return 0, err
	}

	for _, p := range rawProducts {
		if p.Slug == slug {
			unitCostUSD := p.PriceUSD
			for _, tier := range p.Tiers {
				if qty >= tier.MinQty && (tier.MaxQty == nil || qty <= *tier.MaxQty) {
					unitCostUSD = tier.PriceUSD
					break
				}
			}
			return unitCostUSD, nil
		}
	}

	return 0, fmt.Errorf("produk '%s' tidak ditemukan di upstream", slug)
}

func extractFloat(val interface{}) (float64, bool) {
	if val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		clean := strings.TrimSpace(v)
		clean = strings.TrimPrefix(clean, "$")
		clean = strings.TrimSpace(clean)
		if f, err := strconv.ParseFloat(clean, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func extractString(val interface{}) string {
	if val == nil {
		return "0"
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseDigitalCoreUserMe(bodyBytes []byte) (balance float64, totalOrders string, err error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return 0, "0", fmt.Errorf("failed to decode JSON response: %w", err)
	}

	lookupMaps := []map[string]interface{}{raw}
	for _, nestedKey := range []string{"data", "user", "account", "result"} {
		if nested, ok := raw[nestedKey].(map[string]interface{}); ok {
			lookupMaps = append(lookupMaps, nested)
		}
	}

	balanceFound := false
	balanceKeys := []string{"balance", "wallet", "saldo", "credit", "amount"}
	for _, m := range lookupMaps {
		for _, k := range balanceKeys {
			if v, ok := m[k]; ok {
				if f, ok := extractFloat(v); ok {
					balance = f
					balanceFound = true
					break
				}
			}
		}
		if balanceFound {
			break
		}
	}

	orderKeys := []string{"total_orders", "totalOrders", "orders", "order_count", "total"}
	for _, m := range lookupMaps {
		for _, k := range orderKeys {
			if v, ok := m[k]; ok {
				totalOrders = extractString(v)
				break
			}
		}
		if totalOrders != "" && totalOrders != "0" {
			break
		}
	}
	if totalOrders == "" {
		totalOrders = "0"
	}

	return balance, totalOrders, nil
}

// GetUserAccount fetches the current Telegram bot balance and orders count
func (c *DigitalCoreClient) GetUserAccount() (*DigitalCoreMeResponse, error) {
	if c.cfg.DigitalCoreAPIKey == "" {
		return &DigitalCoreMeResponse{Balance: 0, TotalOrders: "0"}, nil
	}

	url := fmt.Sprintf("%s/api/user/me", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("digitalcore /me error %d: %s", resp.StatusCode, string(body))
	}

	balance, totalOrders, err := parseDigitalCoreUserMe(body)
	if err != nil {
		var me DigitalCoreMeResponse
		if uerr := json.Unmarshal(body, &me); uerr == nil {
			return &me, nil
		}
		return nil, err
	}

	return &DigitalCoreMeResponse{
		Balance:     balance,
		TotalOrders: totalOrders,
	}, nil
}

// CheckConnection tests the DigitalCore upstream API and returns detailed diagnostic info
func (c *DigitalCoreClient) CheckConnection() (*models.DigitalCoreCheckResult, error) {
	apiKey := strings.TrimSpace(c.cfg.DigitalCoreAPIKey)
	baseURL := strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://digitalcore.top"
	}

	maskedKey := "(belum diatur di .env)"
	if apiKey != "" {
		if len(apiKey) <= 6 {
			maskedKey = "***"
		} else {
			maskedKey = apiKey[:3] + "..." + apiKey[len(apiKey)-3:]
		}
	}

	result := &models.DigitalCoreCheckResult{
		Success:      false,
		Configured:   apiKey != "",
		BaseURL:      baseURL,
		APIKeyMasked: maskedKey,
		HTTPStatus:   0,
		Balance:      0,
		TotalOrders:  "0",
	}

	if apiKey == "" {
		result.Message = "DIGITALCORE_API_KEY belum dikonfigurasi di file .env VPS."
		return result, fmt.Errorf("%s", result.Message)
	}

	url := fmt.Sprintf("%s/api/user/me", baseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal membuat request: %v", err)
		return result, err
	}

	req.Header.Set("Api-Key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal menghubungi server DigitalCore: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal membaca respon DigitalCore: %v", err)
		return result, err
	}
	result.RawResponse = string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("DigitalCore menolak request (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
		return result, fmt.Errorf("%s", result.Message)
	}

	balance, totalOrders, err := parseDigitalCoreUserMe(bodyBytes)
	if err != nil {
		result.Message = fmt.Sprintf("HTTP 200 OK tetapi gagal parsing format JSON: %v", err)
		return result, err
	}

	result.Success = true
	result.Balance = balance
	result.TotalOrders = totalOrders
	result.Message = fmt.Sprintf("Koneksi DigitalCore sukses! Saldo bot: $%.2f USD | Total order: %s", balance, totalOrders)

	return result, nil
}

// BuyProduct executes the purchase transaction on DigitalCore
func (c *DigitalCoreClient) BuyProduct(slugOrID string, count int) (*DigitalCoreBuyResponse, error) {
	if c.cfg.DigitalCoreAPIKey == "" {
		return nil, fmt.Errorf("DIGITALCORE_API_KEY is not configured")
	}

	targetSlug := strings.TrimSpace(slugOrID)
	// Normalize spaces and dashes to underscores per DigitalCore slug convention (e.g. adobe_express_4_months_trial)
	targetSlug = strings.ReplaceAll(targetSlug, " ", "_")
	targetSlug = strings.ReplaceAll(targetSlug, "-", "_")

	// Match against raw products to get canonical slug
	if rawProds, err := c.FetchRawProducts(); err == nil {
		for _, p := range rawProds {
			if strings.EqualFold(p.Slug, slugOrID) || strings.EqualFold(p.Slug, targetSlug) || strings.EqualFold(p.Name, slugOrID) {
				targetSlug = p.Slug
				break
			}
		}
	}

	baseURL := strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://digitalcore.top"
	}

	// DigitalCore API documentation requires 'id' (string slug e.g. gemini_pro_18_months) and 'count'.
	// Supplying both URL query parameters and JSON body ensures 100% compatibility with upstream server validators.
	buyURL := fmt.Sprintf("%s/api/user/buy?id=%s&count=%d", baseURL, url.QueryEscape(targetSlug), count)
	payload := map[string]interface{}{
		"id":    targetSlug,
		"count": count,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, buyURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error connecting to digitalcore: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var buyResp DigitalCoreBuyResponse
	if err := json.Unmarshal(body, &buyResp); err != nil {
		return nil, fmt.Errorf("failed to parse buy response: %w (body: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK || !buyResp.Success {
		errMsg := buyResp.Msg
		if errMsg == "" {
			errMsg = fmt.Sprintf("status code %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("digitalcore buy failed: %s", errMsg)
	}

	return &buyResp, nil
}
