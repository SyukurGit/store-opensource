package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/repository"
	"storeku/internal/service"
)

type PublicHandler struct {
	cfg            *config.Config
	dcClient       *service.DigitalCoreClient
	orderService   *service.OrderService
	settingRepo    *repository.SettingRepository
	lastCheckTimes map[string]time.Time
	checkMu        sync.Mutex
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
	products, err := h.dcClient.FetchProducts()
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal memuat katalog produk dari upstream: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    products,
	})
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

	product, err := h.dcClient.GetPublicProductBySlug(slug)
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
	if err != nil {
		sendJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    product,
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

func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
