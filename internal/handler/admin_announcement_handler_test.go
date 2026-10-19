package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"storeku/internal/config"
	"storeku/internal/database"
	"storeku/internal/models"
	"storeku/internal/repository"
)

func setupTestDBForAnnouncement(t *testing.T) (*database.DB, *repository.SettingRepository) {
	tempDB := filepath.Join(t.TempDir(), "test_announcement.db")
	db, err := database.InitDB(tempDB)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	repo := repository.NewSettingRepository(db)
	return db, repo
}

func TestAnnouncementFlow(t *testing.T) {
	db, settingRepo := setupTestDBForAnnouncement(t)
	defer db.Close()

	cfg := &config.Config{AdminSecretKey: "test_secret"}
	adminH := NewAdminHandler(cfg, nil, settingRepo, nil, nil, nil, nil)
	publicH := NewPublicHandler(cfg, nil, nil, settingRepo)

	// 1. Initial State: Announcement is inactive or empty
	reqGetPub := httptest.NewRequest(http.MethodGet, "/api/announcement", nil)
	wGetPub := httptest.NewRecorder()
	publicH.GetAnnouncement(wGetPub, reqGetPub)

	if wGetPub.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wGetPub.Code)
	}
	var resPub map[string]interface{}
	_ = json.Unmarshal(wGetPub.Body.Bytes(), &resPub)
	if resPub["data"] != nil {
		t.Errorf("expected null announcement initially, got %v", resPub["data"])
	}

	// 2. Admin saves announcement with bilingual support
	annPayload := models.Announcement{
		IsActive:     true,
		Title:        "PROMO AKHIR BULAN",
		TitleEN:      "END OF MONTH SALE",
		Content:      "Dapatkan diskon hingga 50% untuk semua akun AI premium!",
		ContentEN:    "Get up to 50% discount on all premium AI accounts!",
		BannerURL:    "/logo/gambar/alert.png",
		ActionText:   "Lihat Promo ➔",
		ActionTextEN: "View Offers ➔",
		ActionURL:    "/product?slug=chatgpt-plus",
	}
	payloadBytes, _ := json.Marshal(annPayload)

	reqSave := httptest.NewRequest(http.MethodPost, "/api/admin/announcement", bytes.NewReader(payloadBytes))
	wSave := httptest.NewRecorder()
	adminH.SaveAnnouncement(wSave, reqSave)

	if wSave.Code != http.StatusOK {
		t.Fatalf("expected status 200 on SaveAnnouncement, got %d: %s", wSave.Code, wSave.Body.String())
	}

	var resSave map[string]interface{}
	_ = json.Unmarshal(wSave.Body.Bytes(), &resSave)
	if resSave["success"] != true {
		t.Fatalf("expected success true, got %v", resSave["success"])
	}
	savedData := resSave["data"].(map[string]interface{})
	if savedData["updated_at"] == nil || savedData["updated_at"].(float64) <= 0 {
		t.Errorf("expected updated_at timestamp to be populated, got %v", savedData["updated_at"])
	}

	// 3. Admin gets saved announcement
	reqGetAdmin := httptest.NewRequest(http.MethodGet, "/api/admin/announcement", nil)
	wGetAdmin := httptest.NewRecorder()
	adminH.GetAnnouncement(wGetAdmin, reqGetAdmin)

	if wGetAdmin.Code != http.StatusOK {
		t.Fatalf("expected status 200 on GetAnnouncement, got %d", wGetAdmin.Code)
	}

	// 4. Public gets announcement in Indonesian (default)
	wPubID := httptest.NewRecorder()
	publicH.GetAnnouncement(wPubID, httptest.NewRequest(http.MethodGet, "/api/announcement", nil))
	var resID map[string]interface{}
	_ = json.Unmarshal(wPubID.Body.Bytes(), &resID)
	dataID := resID["data"].(map[string]interface{})
	if dataID["title"] != "PROMO AKHIR BULAN" {
		t.Errorf("expected Indonesian title 'PROMO AKHIR BULAN', got '%v'", dataID["title"])
	}
	if dataID["action_text"] != "Lihat Promo ➔" {
		t.Errorf("expected Indonesian action text, got '%v'", dataID["action_text"])
	}

	// 5. Public gets announcement in English (?lang=en)
	wPubEN := httptest.NewRecorder()
	publicH.GetAnnouncement(wPubEN, httptest.NewRequest(http.MethodGet, "/api/announcement?lang=en", nil))
	var resEN map[string]interface{}
	_ = json.Unmarshal(wPubEN.Body.Bytes(), &resEN)
	dataEN := resEN["data"].(map[string]interface{})
	if dataEN["title"] != "END OF MONTH SALE" {
		t.Errorf("expected English title 'END OF MONTH SALE', got '%v'", dataEN["title"])
	}
	if dataEN["action_text"] != "View Offers ➔" {
		t.Errorf("expected English action text, got '%v'", dataEN["action_text"])
	}

	// 6. Test fallback when English fields are blank
	annFallback := models.Announcement{
		IsActive:   true,
		Title:      "PEMELIHARAAN SISTEM",
		Content:    "Sistem akan maintenance selama 10 menit.",
		BannerURL:  "/logo/gambar/alert.png",
		ActionText: "Kontak CS",
	}
	fbBytes, _ := json.Marshal(annFallback)
	adminH.SaveAnnouncement(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/admin/announcement", bytes.NewReader(fbBytes)))

	wFallback := httptest.NewRecorder()
	publicH.GetAnnouncement(wFallback, httptest.NewRequest(http.MethodGet, "/api/announcement?lang=en", nil))
	var resFB map[string]interface{}
	_ = json.Unmarshal(wFallback.Body.Bytes(), &resFB)
	dataFB := resFB["data"].(map[string]interface{})
	if dataFB["title"] != "PEMELIHARAAN SISTEM" {
		t.Errorf("expected fallback to Indonesian title when EN is empty, got '%v'", dataFB["title"])
	}
	if dataFB["content"] != "Sistem akan maintenance selama 10 menit." {
		t.Errorf("expected fallback to Indonesian content when EN is empty, got '%v'", dataFB["content"])
	}
}
