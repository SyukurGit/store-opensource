package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"storeku/internal/models"
)

// GetDatabaseStats handles GET /api/admin/maintenance/stats
func (h *AdminHandler) GetDatabaseStats(w http.ResponseWriter, r *http.Request) {
	if h.maintenanceRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan pemeliharaan database tidak tersedia",
		})
		return
	}

	stats, err := h.maintenanceRepo.GetStats(h.cfg.DatabasePath)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal mengambil statistik database: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

// CleanDatabase handles POST /api/admin/maintenance/clean
func (h *AdminHandler) CleanDatabase(w http.ResponseWriter, r *http.Request) {
	if h.maintenanceRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan pemeliharaan database tidak tersedia",
		})
		return
	}

	var req models.DatabaseCleanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid: " + err.Error(),
		})
		return
	}

	// Secondary security verification: require admin password confirmation
	if strings.TrimSpace(req.AdminPassword) == "" || req.AdminPassword != h.cfg.AdminPassword {
		sendJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"error":   "Password admin konfirmasi tidak valid atau salah",
		})
		return
	}

	result, err := h.maintenanceRepo.CleanDatabase(h.cfg.DatabasePath, &req)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal melakukan pembersihan database: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pembersihan database SQLite berhasil diselesaikan",
		"data":    result,
	})
}
