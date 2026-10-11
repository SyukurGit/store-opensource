package models

import "time"

// AdminLoginLog represents an entry in the admin login audit log
type AdminLoginLog struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	Location   string    `json:"location"`
	DeviceInfo string    `json:"device_info"`
	LoginAt    time.Time `json:"login_at"`
}

// LastLoginResponse represents response payload for GET /api/admin/last-login
type LastLoginResponse struct {
	LastLogin *AdminLoginLog   `json:"last_login"`
	History   []*AdminLoginLog `json:"history"`
}
