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

type PricingSnapshot struct {
	Rate          float64
	MarginPercent float64
	MarginFixed   int64
}

func (s *PricingService) GetSnapshot() PricingSnapshot {
	return PricingSnapshot{
		Rate:          s.GetUSDRate(),
		MarginPercent: s.GetMarginPercent(),
		MarginFixed:   s.GetMarginFixed(),
	}
}

func (s *PricingService) GetUSDRate() float64 {
	if s.settingRepo == nil {
		if s.cfg != nil && s.cfg.USDToIDRRate > 0 {
			return s.cfg.USDToIDRRate
		}
		return 18000.0
	}
	defaultRate := 18000.0
	if s.cfg != nil && s.cfg.USDToIDRRate > 0 {
		defaultRate = s.cfg.USDToIDRRate
	}
	return s.settingRepo.GetFloat("usd_rate", defaultRate)
}

func (s *PricingService) GetMarginPercent() float64 {
	if s.settingRepo == nil {
		if s.cfg != nil {
			return s.cfg.ProfitMarginPercent
		}
		return 25.0
	}
	defaultPct := 25.0
	if s.cfg != nil {
		defaultPct = s.cfg.ProfitMarginPercent
	}
	return s.settingRepo.GetFloat("margin_percent", defaultPct)
}

func (s *PricingService) GetMarginFixed() int64 {
	if s.settingRepo == nil {
		if s.cfg != nil {
			return s.cfg.ProfitMarginFixed
		}
		return 2000
	}
	defaultFixed := int64(2000)
	if s.cfg != nil {
		defaultFixed = s.cfg.ProfitMarginFixed
	}
	return s.settingRepo.GetInt64("margin_fixed", defaultFixed)
}

func (s *PricingService) GetMaxSurgeDisablePercent() float64 {
	if s.settingRepo == nil {
		return 50.0
	}
	return s.settingRepo.GetFloat("max_surge_disable_percent", 50.0)
}

// ConvertUSDToIDRWithSnapshot calculates selling IDR price in-memory using an existing snapshot (0 database queries).
func (s *PricingService) ConvertUSDToIDRWithSnapshot(usdPrice float64, snap PricingSnapshot) int64 {
	if usdPrice <= 0 {
		return 0
	}
	rate := snap.Rate
	if rate <= 0 {
		rate = 18000.0
	}
	baseIDR := usdPrice * rate
	withPct := baseIDR * (1.0 + (snap.MarginPercent / 100.0))
	finalRaw := withPct + float64(snap.MarginFixed)
	rounded := math.Ceil(finalRaw/100.0) * 100.0
	return int64(rounded)
}

// ConvertUSDToIDR converts wholesale USD price to selling IDR price
func (s *PricingService) ConvertUSDToIDR(usdPrice float64) int64 {
	if usdPrice <= 0 {
		return 0
	}
	snap := s.GetSnapshot()
	return s.ConvertUSDToIDRWithSnapshot(usdPrice, snap)
}
