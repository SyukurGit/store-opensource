package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"storeku/internal/database"
	"storeku/internal/repository"
)

func TestNormalizeWhatsAppNumber(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"0812-3456-7890", "6281234567890", false},
		{"+62 812 3456 7890", "6281234567890", false},
		{"81234567890", "6281234567890", false},
		{"", "", false},
		{"123", "", true},
	}

	for _, tc := range cases {
		got, err := normalizeWhatsAppNumber(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeWhatsAppNumber(%q) expected error, got %q", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeWhatsAppNumber(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("normalizeWhatsAppNumber(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormalizeSocialURL(t *testing.T) {
	tiktok, err := normalizeSocialURL("@digimest", "tiktok")
	if err != nil || tiktok != "https://www.tiktok.com/@digimest" {
		t.Errorf("tiktok normalize = %q, err = %v", tiktok, err)
	}

	threads, err := normalizeSocialURL("threads.net/@digimest", "threads")
	if err != nil || threads != "https://threads.net/@digimest" {
		t.Errorf("threads normalize = %q, err = %v", threads, err)
	}

	if _, err := normalizeSocialURL("https://example.com/@digimest", "tiktok"); err == nil {
		t.Error("expected tiktok domain validation error")
	}

	if _, err := normalizeSocialURL("javascript:alert(1)", "url"); err == nil {
		t.Error("expected invalid scheme error")
	}
}

func TestSaveAndLoadSupportLinks(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := repository.NewSettingRepository(db)
	h := &AdminHandler{settingRepo: repo}

	body := `{
		"whatsapp_number": "0812-3456-7890",
		"tiktok_url": "@digimest",
		"threads_url": "https://www.threads.net/@digimest",
		"whatsapp_social_url": "",
		"reseller_url": ""
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/support-links", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.SaveSupportLinks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			WhatsAppNumber    string `json:"whatsapp_number"`
			WhatsAppURL       string `json:"whatsapp_url"`
			TikTokURL         string `json:"tiktok_url"`
			ThreadsURL        string `json:"threads_url"`
			WhatsAppSocialURL string `json:"whatsapp_social_url"`
			ResellerURL       string `json:"reseller_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}

	if !response.Success {
		t.Fatalf("expected success response")
	}
	if response.Data.WhatsAppNumber != "+6281234567890" {
		t.Errorf("whatsapp_number = %q", response.Data.WhatsAppNumber)
	}
	if response.Data.WhatsAppURL != "https://wa.me/6281234567890" {
		t.Errorf("whatsapp_url = %q", response.Data.WhatsAppURL)
	}
	if response.Data.TikTokURL != "https://www.tiktok.com/@digimest" {
		t.Errorf("tiktok_url = %q", response.Data.TikTokURL)
	}
	if response.Data.WhatsAppSocialURL != "https://wa.me/6281234567890" {
		t.Errorf("whatsapp_social_url fallback = %q", response.Data.WhatsAppSocialURL)
	}
	if response.Data.ResellerURL != "https://wa.me/6281234567890" {
		t.Errorf("reseller_url fallback = %q", response.Data.ResellerURL)
	}

	loaded := loadSupportLinks(repo)
	if loaded.TikTokURL != response.Data.TikTokURL {
		t.Errorf("loaded tiktok_url = %q", loaded.TikTokURL)
	}
}

func TestSaveSupportLinksRejectsInvalidTikTok(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	h := &AdminHandler{settingRepo: repository.NewSettingRepository(db)}
	body := `{"whatsapp_number":"081234567890","tiktok_url":"https://example.com/profile"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/support-links", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.SaveSupportLinks(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
