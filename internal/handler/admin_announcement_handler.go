package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"storeku/internal/models"
)

// GetAnnouncement handles GET /api/admin/announcement
func (h *AdminHandler) GetAnnouncement(w http.ResponseWriter, r *http.Request) {
	if h.settingRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository pengaturan belum terkonfigurasi",
		})
		return
	}

	val := h.settingRepo.Get("site_announcement", "")
	if val == "" {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": models.Announcement{
				IsActive:  false,
				BannerURL: "/logo/gambar/alert.png",
			},
		})
		return
	}

	var ann models.Announcement
	if err := json.Unmarshal([]byte(val), &ann); err != nil {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": models.Announcement{
				IsActive:  false,
				BannerURL: "/logo/gambar/alert.png",
			},
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    ann,
	})
}

// SaveAnnouncement handles POST /api/admin/announcement
func (h *AdminHandler) SaveAnnouncement(w http.ResponseWriter, r *http.Request) {
	if h.settingRepo == nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   "Repository pengaturan belum terkonfigurasi",
		})
		return
	}

	var req models.Announcement
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format data tidak valid: " + err.Error(),
		})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.TitleEN = strings.TrimSpace(req.TitleEN)
	req.Content = strings.TrimSpace(req.Content)
	req.ContentEN = strings.TrimSpace(req.ContentEN)
	req.ActionText = strings.TrimSpace(req.ActionText)
	req.ActionTextEN = strings.TrimSpace(req.ActionTextEN)
	req.ActionURL = strings.TrimSpace(req.ActionURL)
	req.BannerURL = strings.TrimSpace(req.BannerURL)

	if req.BannerURL == "" {
		req.BannerURL = "/logo/gambar/alert.png"
	}
	req.UpdatedAt = time.Now().Unix()

	bytes, err := json.Marshal(req)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal mengonversi data pengumuman: " + err.Error(),
		})
		return
	}

	if err := h.settingRepo.Set("site_announcement", string(bytes)); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal menyimpan ke database: " + err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pengumuman berhasil diperbarui dan dipublikasikan",
		"data":    req,
	})
}

// UploadAnnouncementBanner handles POST /api/admin/announcement/upload
func (h *AdminHandler) UploadAnnouncementBanner(w http.ResponseWriter, r *http.Request) {
	// Batasi ukuran upload banner maksimal 5MB
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Ukuran file terlalu besar (maksimal 5MB)",
		})
		return
	}

	file, header, err := r.FormFile("banner")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "File banner tidak ditemukan: " + err.Error(),
		})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Format file tidak didukung. Harap gunakan gambar PNG, JPG, JPEG, atau WebP",
		})
		return
	}

	uploadDir := filepath.Join("web", "logo", "gambar")
	_ = os.MkdirAll(uploadDir, 0755)

	filename := fmt.Sprintf("banner_announcement_%d%s", time.Now().UnixNano(), ext)
	targetPath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(targetPath)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal membuat file banner di server: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Gagal menyimpan file banner: " + err.Error(),
		})
		return
	}

	relURL := "/logo/gambar/" + filename
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"url":     relURL,
		"message": "File banner berhasil di-upload",
	})
}
