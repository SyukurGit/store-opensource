package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
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
	cfg            *config.Config
	orderRepo      *repository.OrderRepository
	settingRepo    *repository.SettingRepository
	overrideRepo   *repository.ProductOverrideRepository
	loginRepo      *repository.AdminLoginRepository
	dcClient       *service.DigitalCoreClient
	orderService    *service.OrderService
	voucherService  *service.VoucherService
	maintenanceRepo *repository.MaintenanceRepository
	localProductRepo *repository.LocalProductRepository
	reviewRepo       *repository.ReviewRepository
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

func (h *AdminHandler) SetVoucherService(vs *service.VoucherService) {
	h.voucherService = vs
}

func (h *AdminHandler) SetMaintenanceRepo(mr *repository.MaintenanceRepository) {
	h.maintenanceRepo = mr
}

func (h *AdminHandler) SetLocalProductRepo(lpr *repository.LocalProductRepository) {
	h.localProductRepo = lpr
}

func (h *AdminHandler) SetReviewRepo(rr *repository.ReviewRepository) {
	h.reviewRepo = rr
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

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pengaturan harga dan margin berhasil diperbarui",
	})
}

// ListProducts handles GET /api/admin/products
func (h *AdminHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
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
}

// ListEtalaseProducts handles GET /api/admin/etalase-products
// Unified, atomic, and 100% resilient catalog for /admin/reviews etalase table.
// Never returns HTTP 500 even if DigitalCore is offline or disabled.
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

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Produk mandiri berhasil dihapus",
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
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Urutan etalase dan jumlah terjual berhasil diperbarui",
	})
}

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





