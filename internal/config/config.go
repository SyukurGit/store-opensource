package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	DatabasePath        string
	DigitalCoreBaseURL  string
	DigitalCoreAPIKey   string
	PaymentGAPIURL      string
	PaymentGAPIKey      string
	AdminUsername       string
	AdminPassword       string
	AdminSecretKey      string
	USDToIDRRate            float64
	ProfitMarginPercent     float64
	ProfitMarginFixed       int64
	ResendAPIKey            string
	ResendFromEmail         string
	StoreBaseURL            string
	TelegramSupportUsername string
	AllowedOrigins          string
	VoucherSecretKey        string
}

func LoadConfig() *Config {
	// Load .env if present (ignore error if not present in container/prod)
	_ = godotenv.Load()

	cfg := &Config{
		Port:                    getEnv("PORT", "8080"),
		DatabasePath:            getEnv("DATABASE_PATH", "storeku.db"),
		DigitalCoreBaseURL:      getEnv("DIGITALCORE_BASE_URL", "https://digitalcore.top"),
		DigitalCoreAPIKey:       getEnv("DIGITALCORE_API_KEY", ""),
		PaymentGAPIURL:          getEnv("PAYMENTG_API_URL", "https://paymentg.syukurapi.online/api/sandbox"),
		PaymentGAPIKey:          getEnv("PAYMENTG_API_KEY", ""),
		AdminUsername:           getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:           getEnv("ADMIN_PASSWORD", "admin123"),
		AdminSecretKey:          getEnv("ADMIN_SECRET_KEY", "storeku_default_secret_key"),
		USDToIDRRate:            getEnvFloat("USD_TO_IDR_RATE", 18000.0),
		ProfitMarginPercent:     getEnvFloat("PROFIT_MARGIN_PERCENT", 25.0),
		ProfitMarginFixed:       getEnvInt64("PROFIT_MARGIN_FIXED", 2000),
		ResendAPIKey:            getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:         getEnv("RESEND_FROM_EMAIL", "Digimest <orders@digimest.tech>"),
		StoreBaseURL:            getEnv("STORE_BASE_URL", "https://digimest.tech"),
		TelegramSupportUsername: getEnv("TELEGRAM_SUPPORT_USERNAME", "Digimestsupport"),
		AllowedOrigins:          getEnv("ALLOWED_ORIGINS", "https://digimest.tech,https://*.pages.dev,http://localhost:8080,http://127.0.0.1:8080"),
		VoucherSecretKey:        getEnv("VOUCHER_SECRET_KEY", "voucher_sig_salt_default_key"),
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}
