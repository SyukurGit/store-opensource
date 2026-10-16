package silvex

import (
	"fmt"
	"math"
	"strings"

	"storeku/internal/models"
)

// PricingCalculator converts USD to IDR and retrieves the USD exchange rate.
type PricingCalculator interface {
	ConvertUSDToIDR(usd float64) int64
	GetUSDRate() float64
}

// NormalizeDeliveredItems maps Silvex's delivered objects to uniform license string slice.
func NormalizeDeliveredItems(items []DeliveredItem) []string {
	var result []string
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		content := strings.TrimSpace(item.Content)

		if title != "" && content != "" {
			result = append(result, fmt.Sprintf("%s: %s", title, content))
		} else if content != "" {
			result = append(result, content)
		} else if title != "" {
			result = append(result, title)
		}
	}
	if len(result) == 0 {
		return []string{}
	}
	return result
}

// NormalizeProduct maps a Silvex RawProduct into StoreKu's canonical models.Product.
func NormalizeProduct(raw RawProduct, pricing PricingCalculator) *models.Product {
	// 1. Resolve raw identifier
	rawID := strings.TrimSpace(raw.ProductID)
	if rawID == "" && raw.ID != nil {
		rawID = fmt.Sprintf("%v", raw.ID)
	}

	// 2. Resolve name & slug with "silvex_" namespace to avoid collisions
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = strings.TrimSpace(raw.Title)
	}
	if name == "" {
		name = fmt.Sprintf("Silvex Product %s", rawID)
	}

	baseSlug := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	slug := fmt.Sprintf("silvex_%s", baseSlug)

	// 3. Resolve price in USD
	priceUSD := raw.Price
	if priceUSD <= 0 && raw.PriceUSD > 0 {
		priceUSD = raw.PriceUSD
	}

	var sellIDR, modalIDR int64
	if pricing != nil {
		sellIDR = pricing.ConvertUSDToIDR(priceUSD)
		modalIDR = int64(math.Round(priceUSD * pricing.GetUSDRate()))
	}

	prodType := strings.TrimSpace(raw.Type)
	if prodType == "" {
		prodType = strings.TrimSpace(raw.Category)
	}
	if prodType == "" {
		prodType = "AI Account"
	}

	desc := strings.TrimSpace(raw.Description)
	if desc == "" {
		desc = strings.TrimSpace(raw.Desc)
	}

	warranty := strings.TrimSpace(raw.Warranty)
	if warranty == "" {
		warranty = "Full Garansi"
	}

	stock := raw.Stock
	if stock < 0 {
		stock = 0
	}

	return &models.Product{
		RawID:             rawID,
		Slug:              slug,
		Name:              name,
		Type:              prodType,
		Logo:              ResolveAILogo(name, strings.TrimSpace(raw.Logo)),
		Region:            "Global",
		Stock:             stock,
		StockCapped:       false,
		Warranty:          warranty,
		Description:       desc,
		Terms:             "Membeli berarti setuju dengan ketentuan akun Silvex AI.",
		PriceUSD:          priceUSD,
		ModalIDR:          modalIDR,
		PriceIDR:          sellIDR,
		ReferencePriceIDR: sellIDR,
		IsActive:          true,
		Source:            "SILVEX",
		DisplayOrder:      0,
		SalesCount:        10,
	}
}
