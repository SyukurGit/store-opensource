package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ListOrders handles GET /api/admin/orders
func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	statusFilter := r.URL.Query().Get("status")
	offset := (page - 1) * limit

	orders, total, err := h.orderRepo.ListOrders(limit, offset, statusFilter)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    orders,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// RetryFulfillment handles POST /api/admin/retry
func (h *AdminHandler) RetryFulfillment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Invoice string `json:"invoice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	if err := h.orderService.RetryFulfillment(body.Invoice); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Fulfillment berhasil diproses ulang dan lisensi berhasil dikirim!",
	})
}

// ForceFulfill handles POST /api/admin/orders/{invoice}/force-fulfill
func (h *AdminHandler) ForceFulfill(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	if err := h.orderService.ForceFulfillOrder(invoice); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Fulfillment berhasil diproses dan lisensi berhasil dikirim!",
	})
}

// MarkRefunded handles POST /api/admin/orders/{invoice}/mark-refunded
func (h *AdminHandler) MarkRefunded(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	if err := h.orderService.MarkRefundedOrder(invoice); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pesanan berhasil ditandai sebagai telah di-refund",
	})
}

// ManualPaidAndFulfill is disabled to prevent unverified manual bypass and ensure transaction integrity
func (h *AdminHandler) ManualPaidAndFulfill(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusForbidden, map[string]interface{}{
		"success": false,
		"error":   "Fitur pembayaran manual telah dinonaktifkan demi keselamatan dan integritas transaksi toko.",
	})
}

// ManualDeliverKeys handles POST /api/admin/orders/{invoice}/manual-deliver-keys
func (h *AdminHandler) ManualDeliverKeys(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice tidak boleh kosong",
		})
		return
	}

	var req struct {
		Keys []string `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid: " + err.Error(),
		})
		return
	}

	if err := h.orderService.ManualDeliverKeys(invoice, req.Keys); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Serial key berhasil dikirimkan ke pembeli",
	})
}

// DeleteOrder handles DELETE /api/admin/orders/{invoice}
func (h *AdminHandler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice tidak boleh kosong",
		})
		return
	}

	if err := h.orderRepo.Delete(invoice); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Pesanan '%s' berhasil dihapus dari database", invoice),
	})
}
