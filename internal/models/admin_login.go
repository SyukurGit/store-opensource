package models

import "time"

// AdminLoginLog represents an entry in the admin login audit log
type AdminLoginLog struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent"`
	Location      string    `json:"location"`
	DeviceInfo    string    `json:"device_info"`
	Status        string    `json:"status"`         // "SUCCESS" or "FAILED"
	FailureReason string    `json:"failure_reason"` // e.g. "Password salah", "Username tidak terdaftar"
	LoginAt       time.Time `json:"login_at"`
}

// SecurityStats represents summary statistics of security and login events
type SecurityStats struct {
	TotalSuccess   int64            `json:"total_success"`
	TotalFailed    int64            `json:"total_failed"`
	LastFailedIP   string           `json:"last_failed_ip"`
	LastFailedAt   *time.Time       `json:"last_failed_at"`
	LastFailedUser string           `json:"last_failed_user"`
	CurrentSession *AdminLoginLog   `json:"current_session"`
}

// LastLoginResponse represents response payload for GET /api/admin/last-login
type LastLoginResponse struct {
	LastLogin *AdminLoginLog   `json:"last_login"`
	History   []*AdminLoginLog `json:"history"`
}
