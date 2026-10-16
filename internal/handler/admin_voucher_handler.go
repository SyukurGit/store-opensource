package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"storeku/internal/models"
)

// ListVouchers handles GET /api/admin/vouchers
func (h *AdminHandler) ListVouchers(w http.ResponseWriter, r *http.Request) {
	if h.voucherService == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan voucher tidak tersedia",
		})
		return
	}

	list, err := h.voucherService.GetAllVouchers()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Enrich with signature validity check
	type VoucherItem struct {
		*models.Voucher
		SignatureValid bool `json:"signature_valid"`
	}

	var enriched []VoucherItem
	for _, v := range list {
		enriched = append(enriched, VoucherItem{
			Voucher:        v,
			SignatureValid: h.voucherService.VerifySignature(v),
		})
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    enriched,
	})
}

// CreateVoucher handles POST /api/admin/vouchers
func (h *AdminHandler) CreateVoucher(w http.ResponseWriter, r *http.Request) {
	if h.voucherService == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan voucher tidak tersedia",
		})
		return
	}

	var req models.CreateVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	v, err := h.voucherService.CreateVoucher(&req)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Voucher '%s' berhasil diterbitkan dan ditandatangani!", v.Code),
		"data":    v,
	})
}

// ToggleVoucher handles POST /api/admin/vouchers/{id}/toggle
func (h *AdminHandler) ToggleVoucher(w http.ResponseWriter, r *http.Request) {
	if h.voucherService == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan voucher tidak tersedia",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID voucher tidak valid",
		})
		return
	}

	if err := h.voucherService.ToggleActive(id); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Status voucher berhasil diperbarui",
	})
}

// DeleteVoucher handles DELETE /api/admin/vouchers/{id}
func (h *AdminHandler) DeleteVoucher(w http.ResponseWriter, r *http.Request) {
	if h.voucherService == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan voucher tidak tersedia",
		})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "ID voucher tidak valid",
		})
		return
	}

	if err := h.voucherService.DeleteVoucher(id); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Voucher berhasil dihapus",
	})
}
