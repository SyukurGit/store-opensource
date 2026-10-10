package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
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

// FetchRawProducts fetches all active products directly from DigitalCore and computes baseline IDR
func (c *DigitalCoreClient) FetchRawProducts() ([]*models.Product, error) {
	url := fmt.Sprintf("%s/api/public/products", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.cfg.DigitalCoreAPIKey != "" {
		req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("digitalcore returned status %d: %s", resp.StatusCode, string(body))
	}

	var rawProds []DigitalCoreRawProduct
	if err := json.NewDecoder(resp.Body).Decode(&rawProds); err != nil {
		return nil, fmt.Errorf("failed to decode products: %w", err)
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

				// If promo price equals or exceeds reference price, banner promo turns off!
				if promoCandidate >= refPrice {
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
			IsCustomized:        false,
			Tiers:               p.Tiers,
		}

		maxSurgeDisablePct := c.pricing.GetMaxSurgeDisablePercent()

		if ov, hasOv := overrides[p.Slug]; hasOv {
			item.IsActive = ov.IsActive
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

				if promoCandidate >= refPrice {
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("digitalcore /me error %d: %s", resp.StatusCode, string(body))
	}

	var me DigitalCoreMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return nil, err
	}

	return &me, nil
}

// BuyProduct executes the purchase transaction on DigitalCore
func (c *DigitalCoreClient) BuyProduct(slugOrID string, count int) (*DigitalCoreBuyResponse, error) {
	if c.cfg.DigitalCoreAPIKey == "" {
		return nil, fmt.Errorf("DIGITALCORE_API_KEY is not configured")
	}

	url := fmt.Sprintf("%s/api/user/buy", strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/"))
	payload := map[string]interface{}{
		"id":    slugOrID,
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
