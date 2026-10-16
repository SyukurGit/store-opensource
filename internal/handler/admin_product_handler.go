package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"storeku/internal/models"
)

// ListProducts handles GET /api/admin/products
func (h *AdminHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	if h.dcClient != nil {
		products, err := h.dcClient.FetchAdminProducts()
		if err != nil {
			log.Printf("[ADMIN PRODUCTS] FetchAdminProducts failed: %v", err)
			sendJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
				"data":    []*models.AdminProductItem{},
				"warning": "Katalog upstream DigitalCore tidak dapat dijangkau: " + err.Error(),
			})
			return
		}

		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    products,
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    []*models.AdminProductItem{},
	})
}

// ListEtalaseProducts handles GET /api/admin/etalase-products
// Unified, atomic, and 100% resilient catalog for /admin/reviews etalase table.
// Never returns HTTP 500 even if upstream is offline or disabled.
func (h *AdminHandler) ListEtalaseProducts(w http.ResponseWriter, r *http.Request) {
	var items []*models.AdminEtalaseProductItem
	seenSlugs := make(map[string]bool)

	// 1. Always load local products from SQLite (instant & 100% reliable)
	if h.localProductRepo != nil {
		if locals, err := h.localProductRepo.GetAll(); err == nil {
			for _, lp := range locals {
				sales := lp.SalesCount
				if sales <= 0 {
					sales = 10
				}
				items = append(items, &models.AdminEtalaseProductItem{
					Slug:         lp.Slug,
					Name:         lp.Name,
					OriginalName: lp.Name,
					Type:         lp.Type,
					Logo:         lp.Logo,
					Region:       lp.Region,
					IsActive:     lp.IsActive,
					DisplayOrder: lp.DisplayOrder,
					SalesCount:   sales,
					Source:       "LOCAL",
				})
				seenSlugs[lp.Slug] = true
			}
		}
	}

	// 2. Check if DigitalCore is enabled in settings
	dcEnabled := true
	if h.settingRepo != nil {
		dcEnabled = h.settingRepo.Get("digitalcore_enabled", "true") == "true"
	}

	// 3. If enabled, attempt upstream fetch with strict 3.5s timeout & DB cache fallback
	if dcEnabled && h.dcClient != nil {
		type upstreamRes struct {
			items []*models.AdminProductItem
			err   error
		}
		ch := make(chan upstreamRes, 1)
		go func() {
			prods, err := h.dcClient.FetchAdminProducts()
			ch <- upstreamRes{items: prods, err: err}
		}()

		var upItems []*models.AdminProductItem
		select {
		case res := <-ch:
			if res.err == nil {
				upItems = res.items
			} else {
				log.Printf("[ETALASE] DigitalCore upstream error: %v. Fallback to cached catalog.", res.err)
			}
		case <-time.After(3500 * time.Millisecond):
			log.Printf("[ETALASE] DigitalCore upstream timed out (3.5s). Proceeding with cached/local.")
		}

		// If upstream call failed or timed out, attempt to get cached products from DB/in-memory
		if len(upItems) == 0 {
			if rawCached, err := h.dcClient.FetchRawProducts(); err == nil && len(rawCached) > 0 {
				overrides := make(map[string]*models.ProductOverride)
				if h.overrideRepo != nil {
					if ovs, err := h.overrideRepo.GetAll(); err == nil {
						overrides = ovs
					}
				}
				for _, p := range rawCached {
					name := p.Name
					pType := p.Type
					active := true
					dispOrder := 0
					sales := 10
					if ov, ok := overrides[p.Slug]; ok {
						if ov.CustomName != "" {
							name = ov.CustomName
						}
						if ov.CustomType != "" {
							pType = ov.CustomType
						}
						active = ov.IsActive
						dispOrder = ov.DisplayOrder
						if ov.SalesCount > 0 {
							sales = ov.SalesCount
						}
					}
					if !seenSlugs[p.Slug] {
						items = append(items, &models.AdminEtalaseProductItem{
							Slug:         p.Slug,
							Name:         name,
							OriginalName: p.Name,
							Type:         pType,
							Logo:         p.Logo,
							Region:       p.Region,
							IsActive:     active,
							DisplayOrder: dispOrder,
							SalesCount:   sales,
							Source:       "DIGITALCORE",
						})
						seenSlugs[p.Slug] = true
					}
				}
			}
		} else {
			for _, p := range upItems {
				if !seenSlugs[p.Slug] {
					sales := p.SalesCount
					if sales <= 0 {
						sales = 10
					}
					items = append(items, &models.AdminEtalaseProductItem{
						Slug:         p.Slug,
						Name:         p.Name,
						OriginalName: p.OriginalName,
						Type:         p.Type,
						Logo:         p.Logo,
						Region:       p.Region,
						IsActive:     p.IsActive,
						DisplayOrder: p.DisplayOrder,
						SalesCount:   sales,
						Source:       "DIGITALCORE",
					})
					seenSlugs[p.Slug] = true
				}
			}
		}
	}

	// 4. Sort: display_order > 0 ascending first, followed by default (0), then by Name
	sort.SliceStable(items, func(i, j int) bool {
		aOrder, bOrder := items[i].DisplayOrder, items[j].DisplayOrder
		if aOrder > 0 && bOrder > 0 {
			return aOrder < bOrder
		}
		if aOrder > 0 && bOrder == 0 {
			return true
		}
		if aOrder == 0 && bOrder > 0 {
			return false
		}
		return items[i].Name < items[j].Name
	})

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    items,
	})
}

// SaveProductOverride handles POST /api/admin/products/{slug}
func (h *AdminHandler) SaveProductOverride(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Slug produk tidak boleh kosong",
		})
		return
	}

	var req models.ProductOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format payload tidak valid: " + err.Error(),
		})
		return
	}

	// Fetch upstream product to validate price floors
	rawProducts, err := h.dcClient.FetchRawProducts()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memvalidasi produk upstream: " + err.Error(),
		})
		return
	}

	var rawProd *models.Product
	for _, p := range rawProducts {
		if p.Slug == slug {
			rawProd = p
			break
		}
	}

	if rawProd == nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Produk dengan slug '%s' tidak ditemukan", slug),
		})
		return
	}

	envPriceIDR := rawProd.PriceIDR // default env formula price
	modalIDR := rawProd.ModalIDR    // raw modal in IDR (USD * rate)

	// Validate CustomPriceIDR (Harga Acuan): cannot be lower than default env price
	effectiveRefPrice := envPriceIDR
	if req.CustomPriceIDR > 0 {
		if req.CustomPriceIDR < envPriceIDR {
			sendJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Harga acuan (Rp %d) tidak boleh lebih rendah dari harga default sistem (Rp %d)", req.CustomPriceIDR, envPriceIDR),
			})
			return
		}
		effectiveRefPrice = req.CustomPriceIDR
	}

	// Validate PromoPriceIDR (Harga Promo): must not be lower than modal, and must be lower than reference price
	if req.PromoPriceIDR > 0 {
		if req.PromoPriceIDR < modalIDR {
			sendJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Harga promo (Rp %d) tidak boleh lebih rendah dari modal toko (Rp %d)", req.PromoPriceIDR, modalIDR),
			})
			return
		}
		if req.PromoPriceIDR >= effectiveRefPrice {
			sendJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Harga promo (Rp %d) harus lebih murah dari harga acuan (Rp %d)", req.PromoPriceIDR, effectiveRefPrice),
			})
			return
		}
	}

	override := &models.ProductOverride{
		ProductSlug:       slug,
		CustomName:        req.CustomName,
		CustomType:        req.CustomType,
		CustomRegion:      req.CustomRegion,
		CustomWarranty:    req.CustomWarranty,
		CustomStock:       req.CustomStock,
		CustomPriceIDR:    req.CustomPriceIDR,
		PromoPriceIDR:     req.PromoPriceIDR,
		CustomDescription: req.CustomDescription,
		CustomTerms:       req.CustomTerms,
		DisplayOrder:      req.DisplayOrder,
		SalesCount:        req.SalesCount,
		IsActive:          req.IsActive,
		IsForceOutOfStock: req.IsForceOutOfStock,
		BaselineModalIDR:  modalIDR,
		IsAutoSuspended:   false,
	}

	if err := h.overrideRepo.Upsert(override); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal menyimpan kustomisasi produk: " + err.Error(),
		})
		return
	}

	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Kustomisasi produk berhasil disimpan",
	})
}

// ResetProductOverride handles POST /api/admin/products/{slug}/reset
func (h *AdminHandler) ResetProductOverride(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Slug produk tidak boleh kosong",
		})
		return
	}

	if err := h.overrideRepo.Delete(slug); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal mereset kustomisasi produk: " + err.Error(),
		})
		return
	}

	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Kustomisasi produk berhasil direset ke pengaturan default",
	})
}

// UpdateProductDisplayStats handles PUT /api/admin/products/{slug}/display-stats
func (h *AdminHandler) UpdateProductDisplayStats(w http.ResponseWriter, r *http.Request) {
	rawSlug := chi.URLParam(r, "slug")
	if rawSlug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Slug produk tidak boleh kosong",
		})
		return
	}

	slug := rawSlug
	if unescaped, err := url.PathUnescape(rawSlug); err == nil && unescaped != "" {
		slug = unescaped
	} else if unescaped, err := url.QueryUnescape(rawSlug); err == nil && unescaped != "" {
		slug = unescaped
	}

	var req models.ProductDisplayStatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	if req.SalesCount < 0 {
		req.SalesCount = 0
	}
	if req.DisplayOrder < 0 {
		req.DisplayOrder = 0
	}

	// 1. Try updating local product first if it exists
	isLocal := false
	if h.localProductRepo != nil {
		if lp, err := h.localProductRepo.GetBySlug(slug); err == nil && lp != nil {
			isLocal = true
			if err := h.localProductRepo.UpdateDisplayStats(slug, req.DisplayOrder, req.SalesCount); err != nil {
				sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
					"success": false,
					"error":   "Gagal memperbarui produk lokal: " + err.Error(),
				})
				return
			}
		}
	}

	// 2. If not local, update in product_overrides
	if !isLocal {
		if err := h.overrideRepo.UpdateDisplayStats(slug, req.DisplayOrder, req.SalesCount); err != nil {
			sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   "Gagal memperbarui produk upstream: " + err.Error(),
			})
			return
		}
		if h.dcClient != nil {
			h.dcClient.InvalidateCache()
		}
		if h.catalogService != nil {
			h.catalogService.InvalidateCache()
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Urutan etalase dan jumlah terjual berhasil diperbarui",
	})
}
