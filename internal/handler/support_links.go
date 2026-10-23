package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"storeku/internal/models"
	"storeku/internal/repository"
)

const (
	settingCSWhatsAppNumber = "cs_whatsapp_number"
	settingSocialTikTok     = "social_tiktok_url"
	settingSocialThreads    = "social_threads_url"
	settingSocialWhatsApp   = "social_whatsapp_url"
	settingResellerURL      = "cs_reseller_url"
)

func whatsappDigits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if strings.HasPrefix(digits, "0") {
		digits = "62" + digits[1:]
	} else if strings.HasPrefix(digits, "8") {
		digits = "62" + digits
	}
	return digits
}

func loadSupportLinks(repo *repository.SettingRepository) models.SupportLinks {
	links := models.SupportLinks{}
	if repo == nil {
		return links
	}
	all, err := repo.GetAll()
	if err != nil {
		return links
	}

	digits := whatsappDigits(all[settingCSWhatsAppNumber])
	if digits != "" {
		links.WhatsAppNumber = "+" + digits
		links.WhatsAppURL = "https://wa.me/" + digits
	}
	links.TikTokURL = strings.TrimSpace(all[settingSocialTikTok])
	links.ThreadsURL = strings.TrimSpace(all[settingSocialThreads])
	links.WhatsAppSocialURL = strings.TrimSpace(all[settingSocialWhatsApp])
	links.ResellerURL = strings.TrimSpace(all[settingResellerURL])

	if links.WhatsAppSocialURL == "" {
		links.WhatsAppSocialURL = links.WhatsAppURL
	}
	if links.ResellerURL == "" {
		links.ResellerURL = links.WhatsAppURL
	}
	return links
}

func normalizeWhatsAppNumber(raw string) (string, error) {
	digits := whatsappDigits(raw)
	if digits == "" {
		return "", nil
	}
	if len(digits) < 9 || len(digits) > 15 {
		return "", fmt.Errorf("nomor WhatsApp tidak valid (gunakan format 08xx atau 62xx, 9-15 digit)")
	}
	return digits, nil
}

func normalizeSocialURL(raw, platform string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	if strings.HasPrefix(raw, "@") {
		switch platform {
		case "tiktok":
			raw = "https://www.tiktok.com/" + raw
		case "threads":
			raw = "https://www.threads.net/" + raw
		}
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("link %s tidak valid, gunakan URL lengkap (https://...)", platform)
	}

	host := strings.ToLower(u.Hostname())
	switch platform {
	case "tiktok":
		if host != "tiktok.com" && !strings.HasSuffix(host, ".tiktok.com") {
			return "", fmt.Errorf("link TikTok harus menggunakan domain tiktok.com")
		}
	case "threads":
		if host != "threads.net" && !strings.HasSuffix(host, ".threads.net") &&
			host != "threads.com" && !strings.HasSuffix(host, ".threads.com") {
			return "", fmt.Errorf("link Threads harus menggunakan domain threads.net atau threads.com")
		}
	case "whatsapp":
		if host != "wa.me" && host != "whatsapp.com" && !strings.HasSuffix(host, ".whatsapp.com") {
			return "", fmt.Errorf("link WhatsApp harus menggunakan domain wa.me atau whatsapp.com")
		}
	}
	return u.String(), nil
}

// GetSupportLinks handles GET /api/admin/support-links
func (h *AdminHandler) GetSupportLinks(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    loadSupportLinks(h.settingRepo),
	})
}

// SaveSupportLinks handles POST /api/admin/support-links
func (h *AdminHandler) SaveSupportLinks(w http.ResponseWriter, r *http.Request) {
	var body models.SupportLinks
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format request tidak valid",
		})
		return
	}

	digits, err := normalizeWhatsAppNumber(body.WhatsAppNumber)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	tiktokURL, err := normalizeSocialURL(body.TikTokURL, "tiktok")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	threadsURL, err := normalizeSocialURL(body.ThreadsURL, "threads")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	whatsAppSocialURL, err := normalizeSocialURL(body.WhatsAppSocialURL, "whatsapp")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	resellerURL, err := normalizeSocialURL(body.ResellerURL, "url")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	values := map[string]string{
		settingCSWhatsAppNumber: digits,
		settingSocialTikTok:     tiktokURL,
		settingSocialThreads:    threadsURL,
		settingSocialWhatsApp:   whatsAppSocialURL,
		settingResellerURL:      resellerURL,
	}
	for key, value := range values {
		if err := h.settingRepo.Set(key, value); err != nil {
			sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"error":   "Gagal menyimpan pengaturan: " + err.Error(),
			})
			return
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Link bantuan & media sosial berhasil diperbarui",
		"data":    loadSupportLinks(h.settingRepo),
	})
}
