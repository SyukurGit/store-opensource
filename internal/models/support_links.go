package models

type SupportLinks struct {
	WhatsAppNumber   string `json:"whatsapp_number"`
	WhatsAppURL      string `json:"whatsapp_url"`
	TelegramUsername string `json:"telegram_username"`
	TelegramURL      string `json:"telegram_url"`
	TikTokURL        string `json:"tiktok_url"`
	ThreadsURL       string `json:"threads_url"`
	ResellerURL      string `json:"reseller_url"`
}
