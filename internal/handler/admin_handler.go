package handler

import (
	"time"

	"storeku/internal/config"
	"storeku/internal/provider"
	"storeku/internal/repository"
	"storeku/internal/service"
)

type AdminHandler struct {
	cfg              *config.Config
	orderRepo        *repository.OrderRepository
	settingRepo      *repository.SettingRepository
	overrideRepo     *repository.ProductOverrideRepository
	loginRepo        *repository.AdminLoginRepository
	dcClient         *service.DigitalCoreClient
	orderService     *service.OrderService
	voucherService   *service.VoucherService
	maintenanceRepo  *repository.MaintenanceRepository
	localProductRepo *repository.LocalProductRepository
	reviewRepo       *repository.ReviewRepository
	loginLimiter     *LoginAttemptTracker
	catalogService   *service.CatalogService
	providerRegistry *provider.Registry
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
		loginLimiter: NewLoginAttemptTracker(10, 15*time.Minute, 15*time.Minute),
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

func (h *AdminHandler) SetCatalogService(cs *service.CatalogService) {
	h.catalogService = cs
}

func (h *AdminHandler) SetProviderRegistry(pr *provider.Registry) {
	h.providerRegistry = pr
}
