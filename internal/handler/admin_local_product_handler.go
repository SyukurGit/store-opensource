package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"storeku/internal/models"
)

// ListLocalProducts handles GET /api/admin/custom-products
func (h *AdminHandler) ListLocalProducts(w http.ResponseWriter, r *http.Request) {
	if h.localProductRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository produk mandiri belum terkonfigurasi",
		})
		return
	}

	products, err := h.localProductRepo.GetAll()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memuat produk mandiri: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    products,
	})
}

// GetLocalProduct handles GET /api/admin/custom-products/{id}
func (h *AdminHandler) GetLocalProduct(w http.ResponseWriter, r *http.Request) {
	if h.localProductRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository produk mandiri belum terkonfigurasi",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID produk tidak valid",
		})
		return
	}

	p, err := h.localProductRepo.GetByID(id)
	if err != nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Produk mandiri tidak ditemukan",
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    p,
	})
}

// CreateLocalProduct handles POST /api/admin/custom-products
func (h *AdminHandler) CreateLocalProduct(w http.ResponseWriter, r *http.Request) {
	if h.localProductRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository produk mandiri belum terkonfigurasi",
		})
		return
	}

	var req models.LocalProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format data produk tidak valid: " + err.Error(),
		})
		return
	}

	created, err := h.localProductRepo.Create(&req)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Produk mandiri berhasil ditambahkan",
		"data":    created,
	})
}

// UpdateLocalProduct handles PUT /api/admin/custom-products/{id} and POST /api/admin/custom-products/{id}
func (h *AdminHandler) UpdateLocalProduct(w http.ResponseWriter, r *http.Request) {
	if h.localProductRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository produk mandiri belum terkonfigurasi",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID produk tidak valid",
		})
		return
	}

	var req models.LocalProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format data produk tidak valid: " + err.Error(),
		})
		return
	}

	updated, err := h.localProductRepo.Update(id, &req)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Produk mandiri berhasil diperbarui",
		"data":    updated,
	})
}

// DeleteLocalProduct handles DELETE /api/admin/custom-products/{id}
func (h *AdminHandler) DeleteLocalProduct(w http.ResponseWriter, r *http.Request) {
	if h.localProductRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository produk mandiri belum terkonfigurasi",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID produk tidak valid",
		})
		return
	}

	if err := h.localProductRepo.Delete(id); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Produk mandiri berhasil dihapus",
	})
}
