package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"storeku/internal/models"
)

// translateIDToEN translates Indonesian text to English using Google Translate free endpoint
func translateIDToEN(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	endpoint := "https://translate.googleapis.com/translate_a/single?client=gtx&sl=id&tl=en&dt=t&q=" + url.QueryEscape(text)
	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil || resp.StatusCode != http.StatusOK {
		return text // fallback to original text if offline/error
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return text
	}

	var raw []interface{}
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) == 0 {
		return text
	}

	sentences, ok := raw[0].([]interface{})
	if !ok {
		return text
	}

	var sb strings.Builder
	for _, s := range sentences {
		tuple, ok := s.([]interface{})
		if ok && len(tuple) > 0 {
			if str, ok := tuple[0].(string); ok {
				sb.WriteString(str)
			}
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return text
	}
	return result
}

// GetTranslations handles GET /api/admin/translations
func (h *AdminHandler) GetTranslations(w http.ResponseWriter, r *http.Request) {
	var items []*models.ProductTranslationItem

	// 1. Fetch Local Products
	if h.localProductRepo != nil {
		lps, err := h.localProductRepo.GetAll()
		if err == nil {
			for _, lp := range lps {
				isTrans := strings.TrimSpace(lp.NameEN) != "" || strings.TrimSpace(lp.DescriptionEN) != ""
				items = append(items, &models.ProductTranslationItem{
					Slug:          lp.Slug,
					Source:        "LOCAL",
					NameID:        lp.Name,
					TypeID:        lp.Type,
					RegionID:      lp.Region,
					WarrantyID:    lp.Warranty,
					DescriptionID: lp.Description,
					TermsID:       lp.Terms,
					NameEN:        lp.NameEN,
					TypeEN:        lp.TypeEN,
					RegionEN:      lp.RegionEN,
					WarrantyEN:    lp.WarrantyEN,
					DescriptionEN: lp.DescriptionEN,
					TermsEN:       lp.TermsEN,
					IsTranslated:  isTrans,
					IsActive:      lp.IsActive,
				})
			}
		}
	}

	// 2. Fetch Upstream DigitalCore Products if available
	if h.dcClient != nil {
		if dcProducts, err := h.dcClient.FetchAdminProducts(); err == nil {
			localSlugs := make(map[string]bool)
			for _, it := range items {
				localSlugs[it.Slug] = true
			}
			for _, p := range dcProducts {
				if localSlugs[p.Slug] {
					continue // shadowed by local product
				}
				nameID := p.Name
				typeID := p.Type
				regionID := p.Region
				warrantyID := p.Warranty
				descID := p.Description
				termsID := p.Terms

				nameEN := p.CustomNameEN
				typeEN := p.CustomTypeEN
				regionEN := p.CustomRegionEN
				warrantyEN := p.CustomWarrantyEN
				descEN := p.CustomDescriptionEN
				termsEN := p.CustomTermsEN

				isTrans := strings.TrimSpace(nameEN) != "" || strings.TrimSpace(descEN) != ""

				items = append(items, &models.ProductTranslationItem{
					Slug:          p.Slug,
					Source:        "DIGITALCORE",
					NameID:        nameID,
					TypeID:        typeID,
					RegionID:      regionID,
					WarrantyID:    warrantyID,
					DescriptionID: descID,
					TermsID:       termsID,
					NameEN:        nameEN,
					TypeEN:        typeEN,
					RegionEN:      regionEN,
					WarrantyEN:    warrantyEN,
					DescriptionEN: descEN,
					TermsEN:       termsEN,
					IsTranslated:  isTrans,
					IsActive:      p.IsActive,
				})
			}
		}
	}

	// 3. Fetch Upstream Silvex AI Products if available
	if h.providerRegistry != nil {
		if silvexProv, ok := h.providerRegistry.Get("SILVEX"); ok {
			if sProds, err := silvexProv.FetchCatalog(r.Context()); err == nil {
				overrides := make(map[string]*models.ProductOverride)
				if h.overrideRepo != nil {
					if ovs, err := h.overrideRepo.GetAll(); err == nil {
						overrides = ovs
					}
				}
				existingSlugs := make(map[string]bool)
				for _, it := range items {
					existingSlugs[it.Slug] = true
				}
				for _, p := range sProds {
					if existingSlugs[p.Slug] {
						continue
					}
					nameID := p.Name
					typeID := p.Type
					regionID := p.Region
					warrantyID := p.Warranty
					descID := p.Description
					termsID := p.Terms
					isActive := p.IsActive

					nameEN := ""
					typeEN := ""
					regionEN := ""
					warrantyEN := ""
					descEN := ""
					termsEN := ""

					if ov, ok := overrides[p.Slug]; ok {
						if ov.CustomName != "" {
							nameID = ov.CustomName
						}
						if ov.CustomType != "" {
							typeID = ov.CustomType
						}
						if ov.CustomRegion != "" {
							regionID = ov.CustomRegion
						}
						if ov.CustomWarranty != "" {
							warrantyID = ov.CustomWarranty
						}
						if ov.CustomDescription != "" {
							descID = ov.CustomDescription
						}
						if ov.CustomTerms != "" {
							termsID = ov.CustomTerms
						}
						isActive = ov.IsActive

						nameEN = ov.CustomNameEN
						typeEN = ov.CustomTypeEN
						regionEN = ov.CustomRegionEN
						warrantyEN = ov.CustomWarrantyEN
						descEN = ov.CustomDescriptionEN
						termsEN = ov.CustomTermsEN
					}

					isTrans := strings.TrimSpace(nameEN) != "" || strings.TrimSpace(descEN) != ""

					items = append(items, &models.ProductTranslationItem{
						Slug:          p.Slug,
						Source:        "SILVEX",
						NameID:        nameID,
						TypeID:        typeID,
						RegionID:      regionID,
						WarrantyID:    warrantyID,
						DescriptionID: descID,
						TermsID:       termsID,
						NameEN:        nameEN,
						TypeEN:        typeEN,
						RegionEN:      regionEN,
						WarrantyEN:    warrantyEN,
						DescriptionEN: descEN,
						TermsEN:       termsEN,
						IsTranslated:  isTrans,
						IsActive:      isActive,
					})
				}
			}
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    items,
	})
}

// UpdateTranslation handles PUT /api/admin/translations/{slug}
func (h *AdminHandler) UpdateTranslation(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Slug produk tidak boleh kosong",
		})
		return
	}

	var req models.UpdateTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format data request tidak valid",
		})
		return
	}

	var updated bool
	// 1. Try local product first
	if h.localProductRepo != nil {
		if err := h.localProductRepo.UpdateTranslation(slug, &req); err == nil {
			updated = true
		}
	}

	// 2. Try override repo if not local
	if !updated && h.overrideRepo != nil {
		if err := h.overrideRepo.UpdateTranslation(slug, &req); err == nil {
			updated = true
		}
	}

	if !updated {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Produk tidak ditemukan untuk diperbarui terjemahannya",
		})
		return
	}

	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Terjemahan bahasa Inggris berhasil disimpan",
	})
}

// AutoDraftTranslation handles POST /api/admin/translations/auto-draft
func (h *AdminHandler) AutoDraftTranslation(w http.ResponseWriter, r *http.Request) {
	var req models.AutoDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format data request tidak valid",
		})
		return
	}

	translated := make(map[string]string)
	for key, val := range req.Texts {
		if strings.TrimSpace(val) == "" {
			translated[key] = ""
			continue
		}
		translated[key] = translateIDToEN(val)
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    translated,
	})
}
