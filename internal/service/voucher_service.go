package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"storeku/internal/config"
	"storeku/internal/models"
	"storeku/internal/repository"
)

var codeRegex = regexp.MustCompile(`^[A-Z0-9_\-]+$`)

type VoucherService struct {
	cfg         *config.Config
	voucherRepo *repository.VoucherRepository
	rateLimits  sync.Map // ip string -> []time.Time
}

func NewVoucherService(cfg *config.Config, voucherRepo *repository.VoucherRepository) *VoucherService {
	return &VoucherService{
		cfg:         cfg,
		voucherRepo: voucherRepo,
	}
}

// GenerateSignature produces an HMAC-SHA256 hash for voucher tamper-resistance
func (s *VoucherService) GenerateSignature(code string, discountIDR, minSpendIDR int64, maxUsage int, expiresAt string) string {
	payload := fmt.Sprintf("%s|%d|%d|%d|%s", code, discountIDR, minSpendIDR, maxUsage, expiresAt)
	key := []byte(s.cfg.VoucherSecretKey + ":" + s.cfg.AdminSecretKey)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature verifies the cryptographic integrity of a voucher
func (s *VoucherService) VerifySignature(v *models.Voucher) bool {
	expStr := ""
	if v.ExpiresAt != nil {
		expStr = v.ExpiresAt.UTC().Format(time.RFC3339)
	}
	expected := s.GenerateSignature(v.Code, v.DiscountIDR, v.MinSpendIDR, v.MaxUsage, expStr)
	return hmac.Equal([]byte(expected), []byte(v.Signature))
}

// CheckRateLimit guards against brute-force guessing of voucher codes (max 15 attempts / minute / IP)
func (s *VoucherService) CheckRateLimit(ip string) error {
	now := time.Now()
	val, _ := s.rateLimits.LoadOrStore(ip, []time.Time{})
	times := val.([]time.Time)

	var validTimes []time.Time
	oneMinAgo := now.Add(-1 * time.Minute)
	for _, t := range times {
		if t.After(oneMinAgo) {
			validTimes = append(validTimes, t)
		}
	}

	if len(validTimes) >= 15 {
		return fmt.Errorf("terlalu banyak percobaan validasi voucher. Silakan tunggu 1 menit")
	}

	validTimes = append(validTimes, now)
	s.rateLimits.Store(ip, validTimes)
	return nil
}

// ValidateForCheckout validates a voucher code against a subtotal and calculates the effective discount
func (s *VoucherService) ValidateForCheckout(code string, subtotalIDR int64) (*models.Voucher, int64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, 0, fmt.Errorf("kode voucher tidak boleh kosong")
	}

	v, err := s.voucherRepo.GetByCode(code)
	if err != nil {
		return nil, 0, fmt.Errorf("kode voucher '%s' tidak ditemukan", code)
	}

	// 1. Verify cryptographic HMAC signature
	if !s.VerifySignature(v) {
		return nil, 0, fmt.Errorf("integritas kode voucher tidak sah atau data telah dimodifikasi secara ilegal")
	}

	// 2. Check active flag
	if !v.IsActive {
		return nil, 0, fmt.Errorf("voucher '%s' sedang dinonaktifkan oleh toko", code)
	}

	// 3. Check expiration
	if v.ExpiresAt != nil && v.ExpiresAt.UTC().Before(time.Now().UTC()) {
		return nil, 0, fmt.Errorf("voucher '%s' sudah melewati batas tanggal kedaluwarsa", code)
	}

	// 4. Check quota usage
	if v.UsedCount >= v.MaxUsage {
		return nil, 0, fmt.Errorf("kuota pemakaian voucher '%s' sudah habis terpakai", code)
	}

	// 5. Check minimum spend
	if v.MinSpendIDR > 0 && subtotalIDR < v.MinSpendIDR {
		return nil, 0, fmt.Errorf("voucher '%s' memerlukan minimal total belanja Rp %d", code, v.MinSpendIDR)
	}

	// 6. Calculate effective discount (ShopeePay QRIS requires at least Rp 1,000 final amount)
	maxAllowedDiscount := subtotalIDR - 1000
	if maxAllowedDiscount < 0 {
		maxAllowedDiscount = 0
	}

	effectiveDiscount := v.DiscountIDR
	if effectiveDiscount > maxAllowedDiscount {
		effectiveDiscount = maxAllowedDiscount
	}

	return v, effectiveDiscount, nil
}

// CreateVoucher validates inputs, verifies admin password for secondary authorization, signs, and saves
func (s *VoucherService) CreateVoucher(req *models.CreateVoucherRequest) (*models.Voucher, error) {
	// Secondary authorization: verify admin password
	if req.AdminPassword != s.cfg.AdminPassword {
		return nil, fmt.Errorf("password admin salah! Otorisasi penerbitan voucher ditolak demi keamanan")
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return nil, fmt.Errorf("kode voucher wajib diisi")
	}
	if len(code) < 3 || len(code) > 30 {
		return nil, fmt.Errorf("panjang kode voucher harus antara 3 hingga 30 karakter")
	}
	if !codeRegex.MatchString(code) {
		return nil, fmt.Errorf("kode voucher hanya boleh mengandung huruf, angka, garis bawah (_), atau tanda hubung (-)")
	}

	if req.DiscountIDR < 500 {
		return nil, fmt.Errorf("potongan diskon minimal Rp 500")
	}
	if req.DiscountIDR > 5000000 {
		return nil, fmt.Errorf("potongan diskon maksimal Rp 5.000.000")
	}

	if req.MaxUsage < 1 {
		req.MaxUsage = 1
	}
	if req.MinSpendIDR < 0 {
		req.MinSpendIDR = 0
	}

	var expTime *time.Time
	expStr := ""
	if strings.TrimSpace(req.ExpiresAt) != "" {
		// Attempt parsing RFC3339 or "YYYY-MM-DDTHH:MM" or "YYYY-MM-DD HH:MM"
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			parsed, err = time.Parse("2006-01-02T15:04", req.ExpiresAt)
		}
		if err != nil {
			parsed, err = time.Parse("2006-01-02 15:04", req.ExpiresAt)
		}
		if err != nil {
			return nil, fmt.Errorf("format tanggal kedaluwarsa tidak valid (gunakan format YYYY-MM-DD HH:MM)")
		}
		tUTC := parsed.UTC()
		if tUTC.Before(time.Now().UTC()) {
			return nil, fmt.Errorf("tanggal kedaluwarsa harus di masa mendatang")
		}
		expTime = &tUTC
		expStr = tUTC.Format(time.RFC3339)
	}

	// Compute HMAC signature
	signature := s.GenerateSignature(code, req.DiscountIDR, req.MinSpendIDR, req.MaxUsage, expStr)

	voucher := &models.Voucher{
		Code:        code,
		DiscountIDR: req.DiscountIDR,
		MinSpendIDR: req.MinSpendIDR,
		MaxUsage:    req.MaxUsage,
		UsedCount:   0,
		ExpiresAt:   expTime,
		IsActive:    true,
		Signature:   signature,
		Note:        strings.TrimSpace(req.Note),
	}

	if err := s.voucherRepo.Create(voucher); err != nil {
		return nil, err
	}

	return voucher, nil
}

// GetAllVouchers returns all vouchers from repository
func (s *VoucherService) GetAllVouchers() ([]*models.Voucher, error) {
	return s.voucherRepo.GetAll()
}

// ToggleActive toggles active status of voucher
func (s *VoucherService) ToggleActive(id int64) error {
	return s.voucherRepo.ToggleActive(id)
}

// DeleteVoucher deletes voucher by ID
func (s *VoucherService) DeleteVoucher(id int64) error {
	return s.voucherRepo.Delete(id)
}

// ConsumeVoucherAtomic atomically claims 1 usage quota
func (s *VoucherService) ConsumeVoucherAtomic(code string) (*models.Voucher, error) {
	return s.voucherRepo.ConsumeVoucherAtomic(code)
}

// RevertVoucherUsage reverts 1 usage quota if gateway order fails
func (s *VoucherService) RevertVoucherUsage(code string) error {
	return s.voucherRepo.RevertVoucherUsage(code)
}
