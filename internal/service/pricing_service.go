package service

import (
	"math"

	"storeku/internal/config"
	"storeku/internal/repository"
)

type PricingService struct {
	cfg         *config.Config
	settingRepo *repository.SettingRepository
}

func NewPricingService(cfg *config.Config, settingRepo *repository.SettingRepository) *PricingService {
	return &PricingService{
		cfg:         cfg,
		settingRepo: settingRepo,
	}
}

func (s *PricingService) GetUSDRate() float64 {
	return s.settingRepo.GetFloat("usd_rate", s.cfg.USDToIDRRate)
}

func (s *PricingService) GetMarginPercent() float64 {
	return s.settingRepo.GetFloat("margin_percent", s.cfg.ProfitMarginPercent)
}

func (s *PricingService) GetMarginFixed() int64 {
	return s.settingRepo.GetInt64("margin_fixed", s.cfg.ProfitMarginFixed)
}

func (s *PricingService) GetMaxSurgeDisablePercent() float64 {
	return s.settingRepo.GetFloat("max_surge_disable_percent", 50.0)
}

// ConvertUSDToIDR converts wholesale USD price to selling IDR price
func (s *PricingService) ConvertUSDToIDR(usdPrice float64) int64 {
	if usdPrice <= 0 {
		return 0
	}

	rate := s.GetUSDRate()
	marginPct := s.GetMarginPercent()
	marginFixed := s.GetMarginFixed()

	// 1. Base IDR calculation
	baseIDR := usdPrice * rate

	// 2. Add percentage margin
	withPct := baseIDR * (1.0 + (marginPct / 100.0))

	// 3. Add fixed margin
	finalRaw := withPct + float64(marginFixed)

	// 4. Round up to nearest 100 IDR for clean consumer pricing
	rounded := math.Ceil(finalRaw/100.0) * 100.0

	return int64(rounded)
}
