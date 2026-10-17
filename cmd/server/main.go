package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/handler"
	"storeku/internal/payment"
	"storeku/internal/payment/bep20"
	"storeku/internal/payment/binance"
	"storeku/internal/payment/paymentg"
	"storeku/internal/provider"
	"storeku/internal/provider/digitalcore"
	"storeku/internal/provider/local"
	"storeku/internal/provider/silvex"
	"storeku/internal/repository"
	"storeku/internal/service"
)

func main() {
	log.Println("🚀 Starting StoreKu Digital Goods System...")

	// 1. Load Configuration
	cfg := config.LoadConfig()

	// 2. Initialize Database
	db, err := database.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}

	// 3. Initialize Repositories
	settingRepo := repository.NewSettingRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	overrideRepo := repository.NewProductOverrideRepository(db)
	adminLoginRepo := repository.NewAdminLoginRepository(db)
	voucherRepo := repository.NewVoucherRepository(db)
	maintenanceRepo := repository.NewMaintenanceRepository(db)
	localProductRepo := repository.NewLocalProductRepository(db)
	reviewRepo := repository.NewReviewRepository(db)

	// 4. Initialize Multi-Provider Engine & Catalog
	pricingService := service.NewPricingService(cfg, settingRepo)
	providerRegistry := provider.NewRegistry()
	localProvider := local.NewLocalProvider(localProductRepo)
	dcAdapter := digitalcore.NewAdapter(cfg, pricingService)
	silvexAdapter := silvex.NewAdapter(cfg, pricingService)
	providerRegistry.Register(localProvider)
	providerRegistry.Register(dcAdapter)
	providerRegistry.Register(silvexAdapter)

	catalogService := service.NewCatalogService(providerRegistry, pricingService, overrideRepo, settingRepo, db)
	// Pre-warm catalog cache from local DB (instant < 1ms response for initial visitors)
	catalogService.PrewarmCache()
	// Start background asynchronous catalog sync worker (periodic non-blocking sync every 150s)
	go catalogService.StartBackgroundWorker(context.Background(), 150*time.Second)

	// 5. Initialize Services
	dcClient := service.NewDigitalCoreClient(cfg, pricingService, overrideRepo)
	dcClient.SetDB(db)
	pgClient := service.NewPaymentGClient(cfg)
	resendService := service.NewResendService(cfg)
	voucherService := service.NewVoucherService(cfg, voucherRepo)

	// Payment Gateway Registry (Multi-Payment Ready)
	paymentRegistry := payment.NewRegistry()
	paymentGAdapter := paymentg.NewAdapter(cfg)
	paymentRegistry.Register(paymentGAdapter)
	paymentRegistry.SetDefault("PAYMENTG")

	binanceAdapter := binance.NewAdapter(cfg)
	paymentRegistry.Register(binanceAdapter)
	paymentRegistry.RegisterAlias("PAYMENTG_BINANCE", "BINANCE")

	bep20Adapter := bep20.NewAdapter(cfg)
	paymentRegistry.Register(bep20Adapter)
	paymentRegistry.RegisterAlias("PAYMENTG_BEP20", "USDT_BEP20")
	paymentRegistry.RegisterAlias("BEP20", "USDT_BEP20")

	orderService := service.NewOrderService(orderRepo, pricingService, dcClient, pgClient, resendService)
	orderService.SetVoucherService(voucherService)
	orderService.SetLocalProductRepo(localProductRepo)
	orderService.SetProviderRegistry(providerRegistry)
	orderService.SetCatalogService(catalogService)
	orderService.SetPaymentRegistry(paymentRegistry)

	// 6. Initialize Handlers
	publicHandler := handler.NewPublicHandler(cfg, dcClient, orderService, settingRepo)
	publicHandler.SetVoucherService(voucherService)
	publicHandler.SetLocalProductRepo(localProductRepo)
	publicHandler.SetReviewRepo(reviewRepo)
	publicHandler.SetCatalogService(catalogService)
	webhookHandler := handler.NewWebhookHandler(orderService, pgClient, orderRepo)
	webhookHandler.SetPaymentRegistry(paymentRegistry)
	adminHandler := handler.NewAdminHandler(cfg, orderRepo, settingRepo, overrideRepo, adminLoginRepo, dcClient, orderService)
	adminHandler.SetVoucherService(voucherService)
	adminHandler.SetMaintenanceRepo(maintenanceRepo)
	adminHandler.SetLocalProductRepo(localProductRepo)
	adminHandler.SetReviewRepo(reviewRepo)
	adminHandler.SetCatalogService(catalogService)
	adminHandler.SetProviderRegistry(providerRegistry)

	// Background Sweeper: auto-expire orders that passed their 15-minute deadline
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if count, err := orderService.SweepExpiredOrders(); err == nil && count > 0 {
				log.Printf("[SWEEPER] Auto-expired %d unpaid orders past deadline (reverted vouchers).", count)
			}
		}
	}()

	// 6. Router Setup
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(handler.NewCORSMiddleware(cfg))

	// System Health Check (For VPS Monitoring, Uptime Kuma, Cloudflare health checks)
	r.Get("/health", publicHandler.HealthCheck)

	// Direct Webhook Endpoints (supporting both /webhook/{gateway} and /api/webhook/{gateway})
	r.Post("/webhook/{gateway}", webhookHandler.HandleGatewayWebhook)
	r.Post("/webhook/paymentg", webhookHandler.HandlePaymentGWebhook)
	r.Post("/webhook/bep20", webhookHandler.HandleGatewayWebhook)
	r.Post("/webhook/usdt_bep20", webhookHandler.HandleGatewayWebhook)
	r.Post("/payment/binance-webhook", webhookHandler.HandleGatewayWebhook)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", publicHandler.HealthCheck)
		// Public Store API
		r.Get("/config", publicHandler.GetAppConfig)
		r.Get("/products", publicHandler.GetProducts)
		r.Get("/products/{slug}", publicHandler.GetProductDetail)
		r.Get("/products/{slug}/reviews", publicHandler.GetProductReviews)
		r.Post("/reviews", publicHandler.SubmitReview)
		r.Post("/checkout", publicHandler.CreateCheckout)
		r.Post("/vouchers/validate", publicHandler.ValidateVoucher)
		r.Get("/orders/{invoice}/status", publicHandler.GetOrderStatus)
		r.Post("/orders/{invoice}/check", publicHandler.TriggerCheck)
		r.Post("/orders/{invoice}/cancel", publicHandler.CancelOrder)
		r.Get("/claim", publicHandler.ClaimProduct)

		// Payment Gateways Webhook Receivers (Legacy & Unified Multi-Payment)
		r.Post("/webhook/paymentg", webhookHandler.HandlePaymentGWebhook)
		r.Post("/webhook/{gateway}", webhookHandler.HandleGatewayWebhook)
		r.Post("/webhook/bep20", webhookHandler.HandleGatewayWebhook)
		r.Post("/webhook/usdt_bep20", webhookHandler.HandleGatewayWebhook)
		r.Post("/webhooks/paymentg", webhookHandler.HandleGatewayWebhook)
		r.Post("/payment/binance-webhook", webhookHandler.HandleGatewayWebhook)

		// Admin Auth
		r.Post("/admin/login", adminHandler.Login)
		r.Post("/admin/logout", adminHandler.Logout)

		// Protected Admin Panel
		r.Group(func(r chi.Router) {
			r.Use(handler.AdminAuthMiddleware(cfg))
			r.Get("/admin/stats", adminHandler.GetStats)
			r.Get("/admin/last-login", adminHandler.GetLastLogin)
			r.Get("/admin/security/logs", adminHandler.GetSecurityLogs)
			r.Get("/admin/security/stats", adminHandler.GetSecurityStats)
			r.Get("/admin/digitalcore/check", adminHandler.CheckDigitalCoreConnection)
			r.Post("/admin/digitalcore/check", adminHandler.CheckDigitalCoreConnection)
			r.Get("/admin/silvex/check", adminHandler.CheckSilvexConnection)
			r.Post("/admin/silvex/check", adminHandler.CheckSilvexConnection)
			r.Get("/admin/silvex/products", adminHandler.ListSilvexProducts)
			r.Get("/admin/products", adminHandler.ListProducts)
			r.Get("/admin/etalase-products", adminHandler.ListEtalaseProducts)
			r.Post("/admin/products/{slug}", adminHandler.SaveProductOverride)
			r.Post("/admin/products/{slug}/reset", adminHandler.ResetProductOverride)
			r.Delete("/admin/products/{slug}", adminHandler.ResetProductOverride)
			r.Put("/admin/products/{slug}/display-stats", adminHandler.UpdateProductDisplayStats)
			r.Get("/admin/reviews", adminHandler.ListReviews)
			r.Post("/admin/reviews", adminHandler.CreateReview)
			r.Put("/admin/reviews/{id}", adminHandler.UpdateReview)
			r.Delete("/admin/reviews/{id}", adminHandler.DeleteReview)
			r.Get("/admin/orders", adminHandler.ListOrders)
			r.Post("/admin/retry", adminHandler.RetryFulfillment)
			r.Post("/admin/orders/{invoice}/force-fulfill", adminHandler.ForceFulfill)
			r.Post("/admin/orders/{invoice}/manual-paid-fulfill", adminHandler.ManualPaidAndFulfill)
			r.Post("/admin/orders/{invoice}/mark-refunded", adminHandler.MarkRefunded)
			r.Delete("/admin/orders/{invoice}", adminHandler.DeleteOrder)
			r.Get("/admin/vouchers", adminHandler.ListVouchers)
			r.Post("/admin/vouchers", adminHandler.CreateVoucher)
			r.Post("/admin/vouchers/{id}/toggle", adminHandler.ToggleVoucher)
			r.Delete("/admin/vouchers/{id}", adminHandler.DeleteVoucher)
			r.Get("/admin/settings", adminHandler.GetSettings)
			r.Post("/admin/settings", adminHandler.SaveSettings)
			r.Get("/admin/maintenance/stats", adminHandler.GetDatabaseStats)
			r.Post("/admin/maintenance/clean", adminHandler.CleanDatabase)

			// Local Products Management
			r.Get("/admin/custom-products", adminHandler.ListLocalProducts)
			r.Get("/admin/custom-products/{id}", adminHandler.GetLocalProduct)
			r.Post("/admin/custom-products", adminHandler.CreateLocalProduct)
			r.Put("/admin/custom-products/{id}", adminHandler.UpdateLocalProduct)
			r.Post("/admin/custom-products/{id}", adminHandler.UpdateLocalProduct)
			r.Delete("/admin/custom-products/{id}", adminHandler.DeleteLocalProduct)
			r.Post("/admin/orders/{invoice}/manual-deliver-keys", adminHandler.ManualDeliverKeys)

			// Multilingual Translations Management
			r.Get("/admin/translations", adminHandler.GetTranslations)
			r.Put("/admin/translations/{slug}", adminHandler.UpdateTranslation)
			r.Post("/admin/translations/auto-draft", adminHandler.AutoDraftTranslation)
		})
	})

	// 7. Clean URL Page Handlers & Static Serving
	workDir, _ := os.Getwd()
	webDir := filepath.Join(workDir, "web")

	// Static Assets (css, js, logo)
	r.Handle("/css/*", http.StripPrefix("/css/", http.FileServer(http.Dir(filepath.Join(webDir, "css")))))
	r.Handle("/js/*", http.StripPrefix("/js/", http.FileServer(http.Dir(filepath.Join(webDir, "js")))))
	r.Handle("/logo/*", http.StripPrefix("/logo/", http.FileServer(http.Dir(filepath.Join(webDir, "logo")))))
	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "robots.txt"))
	})
	r.Get("/version.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeFile(w, r, filepath.Join(webDir, "version.json"))
	})

	// 301 Permanent Redirects for legacy .html URLs
	cleanRedirects := map[string]string{
		"/index.html":          "/",
		"/product.html":        "/product",
		"/payment.html":        "/payment",
		"/claim.html":          "/claim",
		"/reviews.html":        "/reviews",
		"/about.html":          "/about",
		"/how-to-buy.html":     "/how-to-buy",
		"/warranty.html":       "/warranty",
		"/privacy.html":        "/privacy",
		"/terms.html":          "/terms",
		"/en/index.html":       "/en/",
		"/en/product.html":     "/en/product",
		"/en/payment.html":     "/en/payment",
		"/en/about.html":       "/en/about",
		"/en/how-to-buy.html":  "/en/how-to-buy",
		"/en/warranty.html":    "/en/warranty",
		"/en/privacy.html":     "/en/privacy",
		"/en/terms.html":       "/en/terms",
		"/en/claim.html":       "/en/claim",
		"/admin/index.html":    "/admin",
		"/admin/login.html":    "/admin/login",
		"/admin/products.html":        "/admin/products",
		"/admin/silvex-products.html": "/admin/silvex-products",
		"/admin/custom-products.html": "/admin/custom-products",
		"/admin/translations.html":    "/admin/translations",
		"/admin/orders.html":          "/admin/orders",
		"/admin/vouchers.html":        "/admin/vouchers",
		"/admin/reviews.html":         "/admin/reviews",
		"/admin/security.html":        "/admin/security",
		"/admin/settings.html":        "/admin/settings",
	}
	for oldPath, newPath := range cleanRedirects {
		target := newPath
		r.Get(oldPath, func(w http.ResponseWriter, r *http.Request) {
			dest := target
			if r.URL.RawQuery != "" {
				dest += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, dest, http.StatusMovedPermanently)
		})
	}

	// Clean Public Page Handlers
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		// Respect explicit user language choice (ID)
		cookie, err := r.Cookie("digimest_lang")
		if err == nil && cookie.Value == "id" {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		// Default to English for international & default landing
		dest := "/en/"
		if r.URL.RawQuery != "" {
			dest += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, dest, http.StatusFound)
	})
	r.Get("/product", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "product.html"))
	})
	r.Get("/payment", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "payment.html"))
	})
	r.Get("/claim", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "claim.html"))
	})
	r.Get("/reviews", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		if slug != "" {
			http.Redirect(w, r, fmt.Sprintf("/product?slug=%s#customerReviewsSection", url.QueryEscape(slug)), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
	r.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "about.html"))
	})
	r.Get("/how-to-buy", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "how-to-buy.html"))
	})
	r.Get("/warranty", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "warranty.html"))
	})
	r.Get("/privacy", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "privacy.html"))
	})
	r.Get("/terms", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "terms.html"))
	})

	// English Clean URL Pages
	r.Get("/en", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/en/", http.StatusMovedPermanently)
	})
	r.Get("/en/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "index.html"))
	})
	r.Get("/en/product", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "product.html"))
	})
	r.Get("/en/payment", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "payment.html"))
	})
	r.Get("/en/about", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "about.html"))
	})
	r.Get("/en/how-to-buy", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "how-to-buy.html"))
	})
	r.Get("/en/warranty", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "warranty.html"))
	})
	r.Get("/en/privacy", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "privacy.html"))
	})
	r.Get("/en/terms", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "terms.html"))
	})
	r.Get("/en/claim", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "en", "claim.html"))
	})

	// Clean Admin Page Handlers
	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "index.html"))
	})
	r.Get("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusMovedPermanently)
	})
	r.Get("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		expectedToken := handler.GenerateAdminToken(cfg.AdminSecretKey)
		cookie, err := r.Cookie("digimest_admin_session")
		if err != nil {
			cookie, err = r.Cookie("digihub_admin_session")
		}
		if err != nil {
			cookie, err = r.Cookie("storeku_admin_session")
		}
		if err == nil && cookie.Value == expectedToken {
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "login.html"))
	})
	r.Get("/admin/products", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "products.html"))
	})
	r.Get("/admin/silvex-products", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "silvex-products.html"))
	})
	r.Get("/admin/custom-products", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "custom-products.html"))
	})
	r.Get("/admin/translations", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "translations.html"))
	})
	r.Get("/admin/reviews", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "reviews.html"))
	})
	r.Get("/admin/orders", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "orders.html"))
	})
	r.Get("/admin/vouchers", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "vouchers.html"))
	})
	r.Get("/admin/security", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "security.html"))
	})
	r.Get("/admin/settings", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "settings.html"))
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("==================================================")
	log.Printf(" Digimest is live on http://localhost:%s (Domain: Digimest.tech)", cfg.Port)
	log.Printf(" 🛍️  Katalog & Toko : http://localhost:%s/", cfg.Port)
	log.Printf(" 🔑  Klaim Lisensi  : http://localhost:%s/claim", cfg.Port)
	log.Printf(" ⚙️  Panel Admin    : http://localhost:%s/admin", cfg.Port)
	log.Printf(" 📡  PaymentG URL   : %s", cfg.PaymentGAPIURL)
	log.Printf(" 📦  DigitalCore URL: %s", cfg.DigitalCoreBaseURL)
	log.Printf(" 📧  Resend Sender  : %s", cfg.ResendFromEmail)
	log.Printf("==================================================")


	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
