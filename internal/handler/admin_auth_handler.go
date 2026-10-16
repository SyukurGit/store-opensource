package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"storeku/internal/models"
)

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

	// 1. Check Rate Limiter (Temporary IP Ban after >10 failures in 15 mins)
	if h.loginLimiter != nil {
		if isBanned, remaining := h.loginLimiter.IsBanned(clientIP); isBanned {
			mins := int(remaining.Minutes()) + 1
			sendJSON(w, http.StatusTooManyRequests, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Terlalu banyak percobaan login gagal dari IP Anda (%s). Akses diblokir sementara selama %d menit demi keamanan.", clientIP, mins),
			})
			return
		}

		// Anti-bot throttling delay if under rapid global attack
		if h.loginLimiter.ShouldThrottleGlobal() {
			time.Sleep(1 * time.Second)
		}
	}

	location := extractLocation(r, clientIP)
	deviceInfo := parseDeviceInfo(r.UserAgent())

	if req.Username != h.cfg.AdminUsername || req.Password != h.cfg.AdminPassword {
		// Truncate raw password if extremely long to protect DB
		rawPwd := req.Password
		if len(rawPwd) > 100 {
			rawPwd = rawPwd[:100] + "..."
		}
		if strings.TrimSpace(rawPwd) == "" {
			rawPwd = "(kosong)"
		}

		var failReason string
		if req.Username != h.cfg.AdminUsername {
			failReason = fmt.Sprintf("Username '%s' tidak terdaftar • Password dicoba: '%s'", req.Username, rawPwd)
		} else {
			failReason = fmt.Sprintf("Password salah • Password dicoba: '%s'", rawPwd)
		}

		if h.loginLimiter != nil {
			isNowBanned, count, _ := h.loginLimiter.RecordFailure(clientIP)
			if isNowBanned {
				failReason += fmt.Sprintf(" • [IP DIBLOKIR: %d kali gagal]", count)
			}
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

	// Success: Reset rate limiter for this IP
	if h.loginLimiter != nil {
		h.loginLimiter.Reset(clientIP)
	}

	token := GenerateAdminToken(h.cfg.AdminSecretKey)

	// Audit Log: Record successful admin login with IP, location, and device (Password is NEVER logged)
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

	orderDir := r.URL.Query().Get("order")

	logs, total, err := h.loginRepo.GetSecurityLogs(statusFilter, searchQuery, orderDir, limit, offset)
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
			"order":  orderDir,
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
