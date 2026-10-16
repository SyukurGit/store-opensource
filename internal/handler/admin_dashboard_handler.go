package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"storeku/internal/models"
)

// GetStats handles GET /api/admin/stats
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.orderRepo.GetAdminStats()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Fetch upstream provider balance in USD
	if h.dcClient != nil {
		if dcMe, err := h.dcClient.GetUserAccount(); err == nil && dcMe != nil {
			stats.DigitalCoreBalance = dcMe.Balance
			stats.DigitalCoreConnected = true
			if str, ok := dcMe.TotalOrders.(string); ok {
				stats.DigitalCoreOrders = str
			} else {
				stats.DigitalCoreOrders = strconv.Itoa(int(dcMe.Balance))
			}
		} else if err != nil {
			stats.DigitalCoreConnected = false
			stats.DigitalCoreError = err.Error()
		}
	} else if h.providerRegistry != nil {
		if prov, ok := h.providerRegistry.Get("DIGITALCORE"); ok {
			if health, err := prov.CheckHealth(r.Context()); err == nil && health != nil {
				stats.DigitalCoreBalance = health.Balance
				stats.DigitalCoreConnected = health.Success
				stats.DigitalCoreOrders = health.TotalOrders
			} else if err != nil {
				stats.DigitalCoreConnected = false
				stats.DigitalCoreError = err.Error()
			}
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

// CheckDigitalCoreConnection handles GET and POST /api/admin/digitalcore/check
func (h *AdminHandler) CheckDigitalCoreConnection(w http.ResponseWriter, r *http.Request) {
	var result *models.DigitalCoreCheckResult
	var err error

	if h.dcClient != nil {
		result, err = h.dcClient.CheckConnection()
	} else if h.providerRegistry != nil {
		if prov, ok := h.providerRegistry.Get("DIGITALCORE"); ok {
			health, hErr := prov.CheckHealth(r.Context())
			if health != nil {
				result = &models.DigitalCoreCheckResult{
					Success:      health.Success,
					Configured:   health.Configured,
					BaseURL:      "https://digitalcore.top",
					APIKeyMasked: "***",
					HTTPStatus:   health.HTTPStatus,
					Balance:      health.Balance,
					TotalOrders:  health.TotalOrders,
					RawResponse:  health.RawResponse,
					Message:      health.Message,
				}
			}
			err = hErr
		}
	}

	if result == nil {
		result = &models.DigitalCoreCheckResult{
			Success: false,
			Message: "Penyedia DigitalCore tidak terkonfigurasi",
		}
	}

	if err != nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"data":    result,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": result.Success,
		"data":    result,
		"message": result.Message,
	})
}

// GetSettings handles GET /api/admin/settings
func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	all, _ := h.settingRepo.GetAll()

	// Fill defaults if empty
	if _, ok := all["usd_rate"]; !ok {
		all["usd_rate"] = strconv.FormatFloat(h.cfg.USDToIDRRate, 'f', -1, 64)
	}
	if _, ok := all["margin_percent"]; !ok {
		all["margin_percent"] = strconv.FormatFloat(h.cfg.ProfitMarginPercent, 'f', -1, 64)
	}
	if _, ok := all["margin_fixed"]; !ok {
		all["margin_fixed"] = strconv.FormatInt(h.cfg.ProfitMarginFixed, 10)
	}
	if _, ok := all["max_surge_disable_percent"]; !ok {
		all["max_surge_disable_percent"] = "50"
	}
	if _, ok := all["telegram_cs_username"]; !ok {
		teleUser := "Digimestsupport"
		if h.cfg != nil && h.cfg.TelegramSupportUsername != "" {
			teleUser = h.cfg.TelegramSupportUsername
		}
		all["telegram_cs_username"] = teleUser
	}
	if _, ok := all["digitalcore_enabled"]; !ok {
		all["digitalcore_enabled"] = "true"
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    all,
	})
}

// SaveSettings handles POST /api/admin/settings
func (h *AdminHandler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	validKeys := map[string]bool{
		"usd_rate":                  true,
		"margin_percent":            true,
		"margin_fixed":              true,
		"max_surge_disable_percent": true,
		"telegram_cs_username":      true,
		"digitalcore_enabled":       true,
	}

	for k, v := range body {
		if validKeys[k] {
			_ = h.settingRepo.Set(k, v)
		}
	}

	// Invalidate catalog cache if settings changed
	if h.catalogService != nil {
		h.catalogService.InvalidateCache()
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pengaturan harga dan margin berhasil diperbarui",
	})
}
