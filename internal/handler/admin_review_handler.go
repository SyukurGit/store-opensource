package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"storeku/internal/models"
)

// ListReviews handles GET /api/admin/reviews
func (h *AdminHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    []*models.ProductReview{},
		})
		return
	}

	limit := 100
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug != "" {
		if unescaped, uerr := url.PathUnescape(slug); uerr == nil && unescaped != "" {
			slug = unescaped
		}
	}
	var reviews []*models.ProductReview
	var err error
	if slug != "" {
		reviews, err = h.reviewRepo.GetByProductSlug(slug, false, limit, offset)
	} else {
		reviews, err = h.reviewRepo.GetAll(limit, offset)
	}

	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memuat ulasan: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    reviews,
	})
}

// CreateReview handles POST /api/admin/reviews
func (h *AdminHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Fitur ulasan sedang tidak aktif",
		})
		return
	}

	var req models.AdminReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	req.ProductSlug = strings.TrimSpace(req.ProductSlug)
	if req.ProductSlug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Produk wajib dipilih",
		})
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Bintang ulasan harus antara 1 sampai 5",
		})
		return
	}

	author := strings.TrimSpace(req.AuthorName)
	if author == "" {
		author = "Pelanggan Terverifikasi"
	}

	rev := &models.ProductReview{
		ProductSlug:  req.ProductSlug,
		OrderInvoice: strings.TrimSpace(req.OrderInvoice),
		AuthorName:   author,
		Rating:       req.Rating,
		ReviewText:   strings.TrimSpace(req.ReviewText),
		IsVisible:    req.IsVisible,
	}

	if err := h.reviewRepo.Create(rev); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal membuat ulasan: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Ulasan berhasil ditambahkan",
		"data":    rev,
	})
}

// UpdateReview handles PUT /api/admin/reviews/{id}
func (h *AdminHandler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Fitur ulasan sedang tidak aktif",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID ulasan tidak valid",
		})
		return
	}

	var req models.AdminReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	rev, err := h.reviewRepo.GetByID(id)
	if err != nil || rev == nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Ulasan tidak ditemukan",
		})
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Bintang ulasan harus antara 1 sampai 5",
		})
		return
	}

	author := strings.TrimSpace(req.AuthorName)
	if author != "" {
		rev.AuthorName = author
	}
	rev.Rating = req.Rating
	rev.ReviewText = strings.TrimSpace(req.ReviewText)
	rev.IsVisible = req.IsVisible

	if err := h.reviewRepo.Update(rev); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memperbarui ulasan: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Ulasan berhasil diperbarui",
		"data":    rev,
	})
}

// DeleteReview handles DELETE /api/admin/reviews/{id}
func (h *AdminHandler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Fitur ulasan sedang tidak aktif",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID ulasan tidak valid",
		})
		return
	}

	if err := h.reviewRepo.Delete(id); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal menghapus ulasan: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Ulasan berhasil dihapus",
	})
}
