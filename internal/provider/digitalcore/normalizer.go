package digitalcore

import (
	"fmt"
	"math"
	"strings"

	"storeku/internal/models"
)

// PricingCalculator defines an interface to calculate IDR conversion and wholesale tiers.
type PricingCalculator interface {
	ConvertUSDToIDR(usd float64) int64
	GetUSDRate() float64
}

// NormalizeRawProduct transforms a DigitalCore raw JSON product into the canonical models.Product.
func NormalizeRawProduct(raw RawProduct, baseURL string, calc PricingCalculator) *models.Product {
	slug := ToSlug(raw.Name)

	var logoURL string
	if raw.Logo != "" {
		if strings.HasPrefix(raw.Logo, "http") {
			logoURL = raw.Logo
		} else {
			logoURL = fmt.Sprintf("%s%s", strings.TrimRight(baseURL, "/"), raw.Logo)
		}
	}

	// Calculate tier pricing in IDR
	var tiers []models.ProductTier
	for _, t := range raw.Tiers {
		var tierIDR int64
		if calc != nil {
			tierIDR = calc.ConvertUSDToIDR(t.Price)
		}
		tiers = append(tiers, models.ProductTier{
			MinQty:   t.MinQty,
			MaxQty:   t.MaxQty,
			PriceUSD: t.Price,
			PriceIDR: tierIDR,
		})
	}

	baseUSD := raw.PriceFrom
	if len(tiers) > 0 && tiers[0].PriceUSD > 0 {
		baseUSD = tiers[0].PriceUSD
	}

	var sellIDR, modalIDR int64
	if calc != nil {
		sellIDR = calc.ConvertUSDToIDR(baseUSD)
		modalIDR = int64(math.Round(baseUSD * calc.GetUSDRate()))
	}

	return &models.Product{
		RawID:             raw.ID,
		Slug:              slug,
		Name:              raw.Name,
		Type:              raw.Type,
		Logo:              logoURL,
		Region:            raw.Region,
		Stock:             raw.Stock,
		StockCapped:       raw.StockCapped,
		Warranty:          raw.Warranty,
		Description:       raw.Desc,
		Terms:             "Membeli berarti setuju",
		PriceUSD:          baseUSD,
		ModalIDR:          modalIDR,
		PriceIDR:          sellIDR,
		ReferencePriceIDR: sellIDR,
		Tiers:             tiers,
		IsActive:          true,
		Source:            "DIGITALCORE",
		DisplayOrder:      0,
		SalesCount:        10,
	}
}

// ToSlug generates a uniform URL/ID slug from a product name.
func ToSlug(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "_")
}
