package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	dcClient     *service.DigitalCoreClient
	orderService *service.OrderService
}

func NewAdminHandler(
	cfg *config.Config,
	orderRepo *repository.OrderRepository,
	settingRepo *repository.SettingRepository,
	overrideRepo *repository.ProductOverrideRepository,
	dcClient *service.DigitalCoreClient,
	orderService *service.OrderService,
) *AdminHandler {
	return &AdminHandler{
		cfg:          cfg,
		orderRepo:    orderRepo,
		settingRepo:  settingRepo,
		overrideRepo: overrideRepo,
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

	if req.Username != h.cfg.AdminUsername || req.Password != h.cfg.AdminPassword {
		sendJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"error":   "Username atau password salah",
		})
		return
	}

	token := GenerateAdminToken(h.cfg.AdminSecretKey)

	// Set persistent HTTP-only session cookie (30 days)
	cookieVal := &http.Cookie{
		Name:     "digihub_admin_session",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookieVal)
	// Also set storeku_admin_session for backward compatibility
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
	http.SetCookie(w, &http.Cookie{
		Name:     "digihub_admin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "storeku_admin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

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
		if str, ok := dcMe.TotalOrders.(string); ok {
			stats.DigitalCoreOrders = str
		} else {
			stats.DigitalCoreOrders = strconv.Itoa(int(dcMe.Balance))
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
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

// ManualPaidAndFulfill handles POST /api/admin/orders/{invoice}/manual-paid-fulfill
func (h *AdminHandler) ManualPaidAndFulfill(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	if err := h.orderService.ManualMarkPaidAndFulfill(invoice); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pesanan berhasil ditandai Lunas dan fulfillment berhasil diproses!",
	})
}

