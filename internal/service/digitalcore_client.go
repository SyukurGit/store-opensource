package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/repository"
)

type DigitalCoreClient struct {
	cfg          *config.Config
	httpClient   *http.Client
	pricing      *PricingService
	overrideRepo *repository.ProductOverrideRepository
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
	defer c.cacheMu.RUnlock()
	if len(c.cachedProds) > 0 {
		log.Printf("[DIGITALCORE FALLBACK] Upstream micro-outage: %v. Serving last cached catalog (%d products).", err, len(c.cachedProds))
		return c.cachedProds, nil
	}
	return nil, err
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
	for _, p := range rawProducts {
		ov, hasOv := overrides[p.Slug]
		if hasOv {
			// If disabled by admin, do not display in public catalog
			if !ov.IsActive {
				continue
			}

			// Force Out of Stock override: product still appears in catalog, but stock is 0 and flagged
			if ov.IsForceOutOfStock {
				p.IsForceOutOfStock = true
				p.Stock = 0
			}

			if ov.CustomName != "" {
				p.Name = ov.CustomName
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

			// Calculate minimum safe price ensuring default profit margin
			minSafePrice := p.PriceIDR // default env price (modal + default margin)

			refPrice := p.PriceIDR
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
		} else {
			p.Terms = "Membeli berarti setuju"
			p.ReferencePriceIDR = p.PriceIDR
			p.PromoPriceIDR = 0
			p.HasPromo = false
			p.DiscountPercent = 0
		}
		p.IsActive = true
		publicProducts = append(publicProducts, p)
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
		envPriceIDR := p.PriceIDR // which was calculated via ConvertUSDToIDR

		item := &models.AdminProductItem{
			Slug:                p.Slug,
			OriginalName:        p.Name,
			Name:                p.Name,
			Type:                p.Type,
			Logo:                p.Logo,
			Region:              p.Region,
			Stock:               p.Stock,
			Warranty:            p.Warranty,
			OriginalPriceUSD:    p.PriceUSD,
			ModalIDR:            modalIDR,
			EnvPriceIDR:         envPriceIDR,
			CalculatedPriceIDR:  envPriceIDR,
			CustomPriceIDR:      0,
			ReferencePriceIDR:   envPriceIDR,
			PromoPriceIDR:       0,
			PriceIDR:            envPriceIDR,
			HasPromo:            false,
			DiscountPercent:     0,
			OriginalDescription: p.Description,
			CustomDescription:   "",
			Description:         p.Description,
			CustomTerms:         "",
			Terms:               "Membeli berarti setuju",
			IsActive:            true,
			IsForceOutOfStock:   false,
			IsOriginalOutOfStock: (p.Stock <= 0),
			IsCustomized:        false,
			Tiers:               p.Tiers,
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

	targetID := interface{}(slugOrID)
	// If slugOrID matches a product slug, resolve to upstream raw ID if available
	if rawProds, err := c.FetchRawProducts(); err == nil {
		for _, p := range rawProds {
			if (p.Slug == slugOrID || strings.EqualFold(p.Name, slugOrID)) && p.RawID != nil {
				targetID = p.RawID
				break
			}
		}
	}

	url := fmt.Sprintf("%s/api/user/buy", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"))
	payload := map[string]interface{}{
		"id":    targetID,
		"count": count,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
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
