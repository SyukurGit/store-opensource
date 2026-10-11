package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"storeku/internal/config"
)

// GenerateAdminToken generates a deterministic signature for admin session
func GenerateAdminToken(secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte("storeku_admin_authenticated_session"))
	return hex.EncodeToString(h.Sum(nil))
}

// AdminAuthMiddleware validates admin session cookie or Authorization header
func AdminAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	expectedToken := GenerateAdminToken(cfg.AdminSecretKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check cookie
			if cookie, err := r.Cookie("digimest_admin_session"); err == nil && cookie.Value == expectedToken {
				next.ServeHTTP(w, r)
				return
			}
			if cookie, err := r.Cookie("digihub_admin_session"); err == nil && cookie.Value == expectedToken {
				next.ServeHTTP(w, r)
				return
			}
			if cookie, err := r.Cookie("storeku_admin_session"); err == nil && cookie.Value == expectedToken {
				next.ServeHTTP(w, r)
				return
			}

			// Check Authorization Bearer
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" && parts[1] == expectedToken {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success": false, "error": "Unauthorized admin access"}`))
		})
	}
}

func isAllowedOrigin(origin, allowedList string) bool {
	if allowedList == "*" || allowedList == "" {
		return true
	}
	originClean := strings.TrimSpace(strings.ToLower(origin))
	for _, p := range strings.Split(allowedList, ",") {
		pattern := strings.TrimSpace(strings.ToLower(p))
		if pattern == "" {
			continue
		}
		if pattern == "*" || pattern == originClean {
			return true
		}
		if strings.Contains(pattern, "*") {
			parts := strings.Split(pattern, "*")
			if len(parts) == 2 && strings.HasPrefix(originClean, parts[0]) && strings.HasSuffix(originClean, parts[1]) {
				return true
			}
		}
	}
	return false
}

// NewCORSMiddleware allows frontend clients and payment webhooks to connect with configurable origins
func NewCORSMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	allowedOrigins := "*"
	if cfg != nil && cfg.AllowedOrigins != "" {
		allowedOrigins = cfg.AllowedOrigins
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if isAllowedOrigin(origin, allowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Webhook-Event, X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware allows frontend clients and payment webhooks to connect (default fallback)
func CORSMiddleware(next http.Handler) http.Handler {
	return NewCORSMiddleware(nil)(next)
}
