package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/repository"
	"storeku/internal/service"
)

type AdminHandler struct {
	cfg          *config.Config
	orderRepo    *repository.OrderRepository
	settingRepo  *repository.SettingRepository
	overrideRepo *repository.ProductOverrideRepository
	loginRepo    *repository.AdminLoginRepository
	dcClient     *service.DigitalCoreClient
	orderService *service.OrderService
}

func NewAdminHandler(
	cfg *config.Config,
	orderRepo *repository.OrderRepository,
	settingRepo *repository.SettingRepository,
	overrideRepo *repository.ProductOverrideRepository,
	loginRepo *repository.AdminLoginRepository,
	dcClient *service.DigitalCoreClient,
	orderService *service.OrderService,
) *AdminHandler {
	return &AdminHandler{
		cfg:          cfg,
		orderRepo:    orderRepo,
		settingRepo:  settingRepo,
		overrideRepo: overrideRepo,
		loginRepo:    loginRepo,
		dcClient:     dcClient,
		orderService: orderService,
	}
}

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login handles POST /api/admin/login
func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	clientIP := extractClientIP(r)
	location := extractLocation(r, clientIP)
	deviceInfo := parseDeviceInfo(r.UserAgent())

	if req.Username != h.cfg.AdminUsername || req.Password != h.cfg.AdminPassword {
		failReason := "Password salah"
		if req.Username != h.cfg.AdminUsername {
			failReason = fmt.Sprintf("Username '%s' tidak terdaftar", req.Username)
		}
		if h.loginRepo != nil {
			_ = h.loginRepo.RecordLogin(req.Username, clientIP, r.UserAgent(), location, deviceInfo, "FAILED", failReason)
		}
		sendJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"error":   "Username atau password salah",
		})
		return
	}

	token := GenerateAdminToken(h.cfg.AdminSecretKey)

	// Audit Log: Record successful admin login with IP, location, and device
	if h.loginRepo != nil {
		_ = h.loginRepo.RecordLogin(req.Username, clientIP, r.UserAgent(), location, deviceInfo, "SUCCESS", "Login berhasil")
	}

	// Set persistent HTTP-only session cookie (30 days)
	cookieVal := &http.Cookie{
		Name:     "digimest_admin_session",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookieVal)
	// Also set backward compatibility cookies
	cookieDigihub := *cookieVal
	cookieDigihub.Name = "digihub_admin_session"
	http.SetCookie(w, &cookieDigihub)
	cookieLegacy := *cookieVal
	cookieLegacy.Name = "storeku_admin_session"
	http.SetCookie(w, &cookieLegacy)

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"token":   token,
		"message": "Login berhasil",
	})
}

// Logout handles POST /api/admin/logout
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	for _, cookieName := range []string{"digimest_admin_session", "digihub_admin_session", "storeku_admin_session"} {
		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logout berhasil",
	})
}


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

	// Fetch upstream DigitalCore balance in USD
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

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

// CheckDigitalCoreConnection handles GET and POST /api/admin/digitalcore/check
func (h *AdminHandler) CheckDigitalCoreConnection(w http.ResponseWriter, r *http.Request) {
	result, err := h.dcClient.CheckConnection()
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
	}

	for k, v := range body {
		if validKeys[k] {
			_ = h.settingRepo.Set(k, v)
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pengaturan harga dan margin berhasil diperbarui",
	})
}

// ListProducts handles GET /api/admin/products
func (h *AdminHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.dcClient.FetchAdminProducts()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memuat produk: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    products,
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
		CustomPriceIDR:    req.CustomPriceIDR,
		PromoPriceIDR:     req.PromoPriceIDR,
		CustomDescription: req.CustomDescription,
		CustomTerms:       req.CustomTerms,
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

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Kustomisasi produk berhasil direset ke pengaturan default",
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

// GetLastLogin handles GET /api/admin/last-login
func (h *AdminHandler) GetLastLogin(w http.ResponseWriter, r *http.Request) {
	clientIP := extractClientIP(r)
	location := extractLocation(r, clientIP)
	deviceInfo := parseDeviceInfo(r.UserAgent())

	if h.loginRepo == nil {
		activeLog := &models.AdminLoginLog{
			Username:   "admin",
			IPAddress:  clientIP,
			Location:   location,
			DeviceInfo: deviceInfo,
			LoginAt:    time.Now(),
		}
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"last_login":    activeLog,
				"current_login": activeLog,
				"history":       []*models.AdminLoginLog{activeLog},
			},
		})
		return
	}

	lastLogin, _ := h.loginRepo.GetLastLogin()
	if lastLogin == nil {
		// Auto-record active session for previously logged-in admin (ensures logs are populated after migration)
		_ = h.loginRepo.RecordLogin("admin", clientIP, r.UserAgent(), location, deviceInfo, "SUCCESS", "Sesi aktif")
		lastLogin, _ = h.loginRepo.GetLastLogin()
	}

	if lastLogin == nil {
		lastLogin = &models.AdminLoginLog{
			Username:      "admin",
			IPAddress:     clientIP,
			Location:      location,
			DeviceInfo:    deviceInfo,
			Status:        "SUCCESS",
			FailureReason: "Sesi aktif",
			LoginAt:       time.Now(),
		}
	}

	history, _ := h.loginRepo.GetLoginHistory(10)
	if len(history) == 0 && lastLogin != nil {
		history = []*models.AdminLoginLog{lastLogin}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"last_login":    lastLogin,
			"current_login": lastLogin,
			"history":       history,
		},
	})
}

// GetSecurityLogs handles GET /api/admin/security/logs
func (h *AdminHandler) GetSecurityLogs(w http.ResponseWriter, r *http.Request) {
	if h.loginRepo == nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"logs":  []interface{}{},
				"total": 0,
			},
		})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	searchQuery := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if off, err := strconv.Atoi(offsetStr); err == nil && off >= 0 {
		offset = off
	}

	logs, total, err := h.loginRepo.GetSecurityLogs(statusFilter, searchQuery, limit, offset)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"logs":   logs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetSecurityStats handles GET /api/admin/security/stats
func (h *AdminHandler) GetSecurityStats(w http.ResponseWriter, r *http.Request) {
	clientIP := extractClientIP(r)
	location := extractLocation(r, clientIP)
	deviceInfo := parseDeviceInfo(r.UserAgent())

	currentSession := &models.AdminLoginLog{
		Username:   h.cfg.AdminUsername,
		IPAddress:  clientIP,
		Location:   location,
		DeviceInfo: deviceInfo,
		Status:     "SUCCESS",
		LoginAt:    time.Now(),
	}

	if h.loginRepo == nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": &models.SecurityStats{
				TotalSuccess:   1,
				TotalFailed:    0,
				CurrentSession: currentSession,
			},
		})
		return
	}

	stats, err := h.loginRepo.GetSecurityStats()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	stats.CurrentSession = currentSession

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

func extractClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0])
		}
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func extractLocation(r *http.Request, clientIP string) string {
	city := strings.TrimSpace(r.Header.Get("CF-IPCity"))
	country := strings.TrimSpace(r.Header.Get("CF-IPCountry"))
	if city != "" && country != "" {
		return fmt.Sprintf("%s, %s", city, country)
	}
	if country != "" {
		return country
	}
	if clientIP == "127.0.0.1" || clientIP == "::1" || strings.HasPrefix(clientIP, "192.168.") || strings.HasPrefix(clientIP, "10.") {
		return "Localhost / Jaringan Lokal"
	}
	return "Indonesia"
}

func parseDeviceInfo(ua string) string {
	if ua == "" {
		return "Unknown Device"
	}
	os := "Unknown OS"
	if strings.Contains(ua, "Windows") {
		os = "Windows"
	} else if strings.Contains(ua, "Android") {
		os = "Android"
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		os = "iOS"
	} else if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS") {
		os = "macOS"
	} else if strings.Contains(ua, "Linux") {
		os = "Linux"
	}

	browser := "Browser"
	if strings.Contains(ua, "Edg/") {
		browser = "Edge"
	} else if strings.Contains(ua, "Chrome/") {
		browser = "Chrome"
	} else if strings.Contains(ua, "Firefox/") {
		browser = "Firefox"
	} else if strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/") {
		browser = "Safari"
	}

	return fmt.Sprintf("%s on %s", browser, os)
}


