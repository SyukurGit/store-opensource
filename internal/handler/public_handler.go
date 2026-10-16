package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/repository"
	"storeku/internal/service"
)

type PublicHandler struct {
	cfg              *config.Config
	dcClient         *service.DigitalCoreClient
	orderService     *service.OrderService
	settingRepo      *repository.SettingRepository
	localProductRepo *repository.LocalProductRepository
	voucherService   *service.VoucherService
	reviewRepo       *repository.ReviewRepository
	catalogService   *service.CatalogService
	lastCheckTimes   map[string]time.Time
	checkMu          sync.Mutex
}

func NewPublicHandler(cfg *config.Config, dcClient *service.DigitalCoreClient, orderService *service.OrderService, settingRepo *repository.SettingRepository) *PublicHandler {
	return &PublicHandler{
		cfg:            cfg,
		dcClient:       dcClient,
		orderService:   orderService,
		settingRepo:    settingRepo,
		lastCheckTimes: make(map[string]time.Time),
	}
}

func (h *PublicHandler) SetVoucherService(vs *service.VoucherService) {
	h.voucherService = vs
}

func (h *PublicHandler) SetLocalProductRepo(repo *repository.LocalProductRepository) {
	h.localProductRepo = repo
}

func (h *PublicHandler) SetReviewRepo(repo *repository.ReviewRepository) {
	h.reviewRepo = repo
}

func (h *PublicHandler) SetCatalogService(cs *service.CatalogService) {
	h.catalogService = cs
}

// HealthCheck handles GET /health and GET /api/health
func (h *PublicHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  "healthy",
		"service": "digimest-backend",
		"version": "1.0.0",
	})
}

// GetAppConfig handles GET /api/config
func (h *PublicHandler) GetAppConfig(w http.ResponseWriter, r *http.Request) {
	username := "Digimestsupport"
	if h.settingRepo != nil {
		if dbUser := h.settingRepo.Get("telegram_cs_username", ""); dbUser != "" {
			username = dbUser
		} else if h.cfg != nil && h.cfg.TelegramSupportUsername != "" {
			username = h.cfg.TelegramSupportUsername
		}
	} else if h.cfg != nil && h.cfg.TelegramSupportUsername != "" {
		username = h.cfg.TelegramSupportUsername
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"telegram_username": username,
		"telegram_url":      fmt.Sprintf("https://t.me/%s", username),
	})
}

// GetProducts handles GET /api/products
func (h *PublicHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")

	// 0. If CatalogService is initialized, use the multi-provider aggregated catalog
	if h.catalogService != nil {
		if prods, err := h.catalogService.GetPublicProducts(r.Context(), lang); err == nil && len(prods) > 0 {
			sendJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
				"data":    prods,
			})
			return
		}
	}

	var localProducts []*models.Product
	if h.localProductRepo != nil {
		if lps, err := h.localProductRepo.GetAllActive(); err == nil {
			for _, lp := range lps {
				localProducts = append(localProducts, lp.ToProduct())
			}
		}
	}

	dcEnabled := true
	if h.settingRepo != nil {
		dcEnabled = h.settingRepo.Get("digitalcore_enabled", "true") == "true"
	}

	// If DigitalCore upstream sync is turned OFF, return only local products immediately
	if !dcEnabled {
		lang := r.URL.Query().Get("lang")
		for _, lp := range localProducts {
			applyLanguageToProduct(lp, lang)
		}
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    localProducts,
		})
		return
	}

	// If DigitalCore is ON, fetch upstream concurrently with a strict 3.5-second timeout
	type dcResult struct {
		products []*models.Product
		err      error
	}
	ch := make(chan dcResult, 1)
	go func() {
		p, err := h.dcClient.FetchProducts()
		ch <- dcResult{products: p, err: err}
	}()

	var dcProducts []*models.Product
	select {
	case res := <-ch:
		if res.err == nil {
			dcProducts = res.products
		}
	case <-time.After(3500 * time.Millisecond):
		// Upstream micro-outage or timeout: gracefully continue with local products only
	}

	// Merge catalog: local products have precedence and shadow any duplicate slug from upstream
	slugSet := make(map[string]bool)
	var merged []*models.Product
	for _, lp := range localProducts {
		slugSet[lp.Slug] = true
		merged = append(merged, lp)
	}
	for _, dp := range dcProducts {
		if !slugSet[dp.Slug] {
			merged = append(merged, dp)
		}
	}

	// Fetch review statistics if repository available
	var statsMap map[string]*models.ReviewStats
	if h.reviewRepo != nil {
		if sm, err := h.reviewRepo.GetAllProductStats(); err == nil {
			statsMap = sm
		}
	}

	for _, p := range merged {
		if p.SalesCount <= 0 {
			p.SalesCount = 10
		}
		if statsMap != nil {
			if st, ok := statsMap[p.Slug]; ok {
				p.RatingAvg = st.AverageRating
				p.RatingCount = st.TotalReviews
			} else {
				p.RatingAvg = 5.0
				p.RatingCount = 0
			}
		} else {
			p.RatingAvg = 5.0
			p.RatingCount = 0
		}
	}

	// Sort catalog by display_order: positive values (> 0) sorted ascending first, followed by default (0)
	sort.SliceStable(merged, func(i, j int) bool {
		aOrder, bOrder := merged[i].DisplayOrder, merged[j].DisplayOrder
		if aOrder > 0 && bOrder > 0 {
			return aOrder < bOrder
		}
		if aOrder > 0 && bOrder == 0 {
			return true
		}
		if aOrder == 0 && bOrder > 0 {
			return false
		}
		return false
	})

	for _, p := range merged {
		applyLanguageToProduct(p, lang)
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    merged,
	})
}

func applyLanguageToProduct(p *models.Product, lang string) {
	if p == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(lang)) == "en" {
		if strings.TrimSpace(p.NameEN) != "" {
			p.Name = p.NameEN
		}
		if strings.TrimSpace(p.TypeEN) != "" {
			p.Type = p.TypeEN
		}
		if strings.TrimSpace(p.RegionEN) != "" {
			p.Region = p.RegionEN
		}
		if strings.TrimSpace(p.WarrantyEN) != "" {
			p.Warranty = p.WarrantyEN
		}
		if strings.TrimSpace(p.DescriptionEN) != "" {
			p.Description = p.DescriptionEN
		}
		if strings.TrimSpace(p.TermsEN) != "" {
			p.Terms = p.TermsEN
		}
	}
}

// GetProductDetail handles GET /api/products/{slug}
func (h *PublicHandler) GetProductDetail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Slug produk tidak boleh kosong",
		})
		return
	}

	lang := r.URL.Query().Get("lang")

	// 0. If CatalogService is available, query centralized multi-provider catalog
	if h.catalogService != nil {
		if p, err := h.catalogService.GetPublicProductBySlug(r.Context(), slug, lang); err == nil && p != nil {
			var stats *models.ReviewStats
			var reviews []*models.ProductReview
			if h.reviewRepo != nil {
				if st, err := h.reviewRepo.GetStatsByProductSlug(p.Slug); err == nil && st != nil {
					stats = st
					p.RatingAvg = st.AverageRating
					p.RatingCount = st.TotalReviews
				}
				if revs, err := h.reviewRepo.GetByProductSlug(p.Slug, true, 50, 0); err == nil && revs != nil {
					reviews = revs
				}
			}
			sendJSON(w, http.StatusOK, map[string]interface{}{
				"success":      true,
				"data":         p,
				"reviews":      reviews,
				"review_stats": stats,
			})
			return
		}
	}

	var product *models.Product

	// 1. Check local products first
	if h.localProductRepo != nil {
		if lp, err := h.localProductRepo.GetBySlug(slug); err == nil && lp != nil && lp.IsActive {
			product = lp.ToProduct()
		} else if unescaped, uerr := url.PathUnescape(slug); uerr == nil && unescaped != slug {
			if lp, err := h.localProductRepo.GetBySlug(unescaped); err == nil && lp != nil && lp.IsActive {
				product = lp.ToProduct()
			}
		}
	}

	// 2. Check DigitalCore catalog if not found in local products
	if product == nil {
		dcEnabled := true
		if h.settingRepo != nil {
			dcEnabled = h.settingRepo.Get("digitalcore_enabled", "true") == "true"
		}
		if !dcEnabled {
			sendJSON(w, http.StatusNotFound, map[string]interface{}{
				"success": false,
				"error":   "Produk tidak ditemukan atau tidak aktif",
			})
			return
		}

		var err error
		product, err = h.dcClient.GetPublicProductBySlug(slug)
		if err != nil {
			if unescaped, uerr := url.PathUnescape(slug); uerr == nil && unescaped != slug {
				product, err = h.dcClient.GetPublicProductBySlug(unescaped)
			}
		}
		if err != nil {
			if unescaped, uerr := url.QueryUnescape(slug); uerr == nil && unescaped != slug {
				product, err = h.dcClient.GetPublicProductBySlug(unescaped)
			}
		}
		if err != nil || product == nil {
			sendJSON(w, http.StatusNotFound, map[string]interface{}{
				"success": false,
				"error":   "Produk tidak ditemukan atau tidak aktif",
			})
			return
		}
	}

	if product.SalesCount <= 0 {
		product.SalesCount = 10
	}

	// Attach reviews and review stats
	var reviews []*models.ProductReview = []*models.ProductReview{}
	var stats *models.ReviewStats = &models.ReviewStats{
		AverageRating: 5.0,
		TotalReviews:  0,
		Distribution:  map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
	}
	if h.reviewRepo != nil {
		if st, err := h.reviewRepo.GetStatsByProductSlug(product.Slug); err == nil && st != nil {
			stats = st
			product.RatingAvg = st.AverageRating
			product.RatingCount = st.TotalReviews
		}
		if revs, err := h.reviewRepo.GetByProductSlug(product.Slug, true, 50, 0); err == nil && revs != nil {
			reviews = revs
		}
	}

	applyLanguageToProduct(product, lang)

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"data":         product,
		"reviews":      reviews,
		"review_stats": stats,
	})
}

// GetProductReviews handles GET /api/products/{slug}/reviews
func (h *PublicHandler) GetProductReviews(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Slug produk wajib disertakan",
		})
		return
	}

	if h.reviewRepo == nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    []*models.ProductReview{},
			"stats":   &models.ReviewStats{AverageRating: 5.0, TotalReviews: 0, Distribution: map[int]int{}},
		})
		return
	}

	stats, err := h.reviewRepo.GetStatsByProductSlug(slug)
	if err != nil {
		stats = &models.ReviewStats{AverageRating: 5.0, TotalReviews: 0, Distribution: map[int]int{}}
	}
	reviews, err := h.reviewRepo.GetByProductSlug(slug, true, 100, 0)
	if err != nil {
		reviews = []*models.ProductReview{}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    reviews,
		"stats":   stats,
	})
}

// SubmitReview handles POST /api/reviews
func (h *PublicHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	if h.reviewRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan ulasan sedang tidak aktif",
		})
		return
	}

	var req models.SubmitReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request ulasan tidak valid",
		})
		return
	}

	req.InvoiceNumber = strings.TrimSpace(req.InvoiceNumber)
	req.ClaimCode = strings.TrimSpace(req.ClaimCode)
	if req.InvoiceNumber == "" || req.ClaimCode == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice dan kode klaim wajib disertakan",
		})
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Bintang rating harus antara 1 sampai 5",
		})
		return
	}

	// Check if already reviewed
	alreadyReviewed, err := h.reviewRepo.HasOrderReviewed(req.InvoiceNumber)
	if err == nil && alreadyReviewed {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Pesanan ini sudah pernah diberikan ulasan",
		})
		return
	}

	order, err := h.orderService.GetOrderByInvoice(req.InvoiceNumber)
	if err != nil || order == nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Pesanan tidak ditemukan",
		})
		return
	}

	if !strings.EqualFold(strings.TrimSpace(order.ClaimCode), req.ClaimCode) {
		sendJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"error":   "Kode klaim tidak sesuai dengan pesanan",
		})
		return
	}

	if order.PaymentStatus != "PAID" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Ulasan hanya dapat dikirim untuk pesanan yang telah lunas",
		})
		return
	}

	author := strings.TrimSpace(req.AuthorName)
	if author == "" {
		if order.CustomerContact != "" {
			author = order.CustomerContact
		} else {
			author = "Pelanggan Terverifikasi"
		}
	}

	rev := &models.ProductReview{
		ProductSlug:  order.ProductSlug,
		OrderInvoice: order.InvoiceNumber,
		AuthorName:   author,
		Rating:       req.Rating,
		ReviewText:   strings.TrimSpace(req.ReviewText),
		IsVisible:    true,
	}

	if err := h.reviewRepo.Create(rev); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal menyimpan ulasan: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Terima kasih! Ulasan Anda berhasil disimpan.",
		"data":    rev,
	})
}

// CreateCheckout handles POST /api/checkout
func (h *PublicHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	var req models.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid: " + err.Error(),
		})
		return
	}

	resp, err := h.orderService.CreateCheckout(&req)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    resp,
	})
}

// GetOrderStatus handles GET /api/orders/{invoice}/status
func (h *PublicHandler) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	status, err := h.orderService.GetOrderStatus(invoice)
	if err != nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

// TriggerCheck handles POST /api/orders/{invoice}/check ("Saya Sudah Bayar")
func (h *PublicHandler) TriggerCheck(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	// Throttling: Maksimal 1 call per 4 detik per invoice agar tidak membanjiri gateway & server
	h.checkMu.Lock()
	// Periodic pruning of throttle cache to avoid unbounded memory growth under high traffic
	if len(h.lastCheckTimes) > 500 {
		cutoff := time.Now().Add(-10 * time.Minute)
		for inv, t := range h.lastCheckTimes {
			if t.Before(cutoff) {
				delete(h.lastCheckTimes, inv)
			}
		}
	}
	lastCheck, exists := h.lastCheckTimes[invoice]
	now := time.Now()
	if exists && now.Sub(lastCheck) < 4*time.Second {
		h.checkMu.Unlock()
		// Kembalikan status terkini tanpa memanggil ulang ke gateway
		status, err := h.orderService.GetOrderStatus(invoice)
		if err != nil {
			sendJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Pemeriksaan mutasi baru saja dilakukan. Mohon tunggu sejenak.",
			})
			return
		}
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Pemeriksaan mutasi baru saja dilakukan.",
			"data":    status,
		})
		return
	}
	h.lastCheckTimes[invoice] = now
	h.checkMu.Unlock()

	if err := h.orderService.TriggerInstantCheck(invoice); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memeriksa status mutasi: " + err.Error(),
		})
		return
	}

	// Fetch updated status
	status, err := h.orderService.GetOrderStatus(invoice)
	if err != nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Pemeriksaan mutasi dikirim",
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

// CancelOrder handles POST /api/orders/{invoice}/cancel
func (h *PublicHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	invoice := chi.URLParam(r, "invoice")
	if invoice == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Nomor invoice wajib disertakan",
		})
		return
	}

	if err := h.orderService.CancelOrder(invoice); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pesanan berhasil dibatalkan",
	})
}

// ClaimProduct handles GET /api/claim?code=...
func (h *PublicHandler) ClaimProduct(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Kode klaim atau nomor invoice wajib diisi",
		})
		return
	}

	res, err := h.orderService.ClaimProduct(code)
	if err != nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, res)
}

// ValidateVoucher handles POST /api/vouchers/validate
func (h *PublicHandler) ValidateVoucher(w http.ResponseWriter, r *http.Request) {
	if h.voucherService == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Layanan voucher tidak tersedia",
		})
		return
	}

	// Rate limiting by client IP
	clientIP := extractClientIP(r)
	if err := h.voucherService.CheckRateLimit(clientIP); err != nil {
		sendJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	var req models.ValidateVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	if req.Quantity < 1 {
		req.Quantity = 1
	}

	// Calculate product subtotal to validate voucher rules against
	products, err := h.dcClient.FetchProducts()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memuat informasi produk",
		})
		return
	}

	var targetProduct *models.Product
	for _, p := range products {
		if p.Slug == req.ProductSlug {
			targetProduct = p
			break
		}
	}

	if targetProduct == nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Produk tidak ditemukan",
		})
		return
	}

	unitPriceIDR := targetProduct.PriceIDR
	subtotalIDR := unitPriceIDR * int64(req.Quantity)

	v, discountIDR, err := h.voucherService.ValidateForCheckout(req.Code, subtotalIDR)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	finalSubtotal := subtotalIDR - discountIDR
	if finalSubtotal < 1000 {
		finalSubtotal = 1000
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": models.ValidateVoucherResponse{
			Valid:             true,
			Code:              v.Code,
			DiscountIDR:       discountIDR,
			DiscountFormatted: fmt.Sprintf("Rp %d", discountIDR),
			SubtotalOriginal:  subtotalIDR,
			SubtotalFinal:     finalSubtotal,
			Message:           fmt.Sprintf("Voucher hemat Rp %d berhasil diterapkan!", discountIDR),
		},
	})
}

func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
