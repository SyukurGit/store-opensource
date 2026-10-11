package repository

import (
	"database/sql"
	"fmt"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
)

type AdminLoginRepository struct {
	db *database.DB
}

func NewAdminLoginRepository(db *database.DB) *AdminLoginRepository {
	return &AdminLoginRepository{db: db}
}

// RecordLogin inserts a new login event into admin_login_logs
func (r *AdminLoginRepository) RecordLogin(username, ipAddress, userAgent, location, deviceInfo string) error {
	query := `
		INSERT INTO admin_login_logs (username, ip_address, user_agent, location, device_info, login_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := r.db.Exec(query, username, ipAddress, userAgent, location, deviceInfo)
	if err != nil {
		return fmt.Errorf("failed to record admin login: %w", err)
	}
	return nil
}

// GetLastLogin returns the most recent admin login record
func (r *AdminLoginRepository) GetLastLogin() (*models.AdminLoginLog, error) {
	query := `
		SELECT id, username, ip_address, user_agent, location, device_info, login_at
		FROM admin_login_logs
		ORDER BY id DESC
		LIMIT 1
	`
	var log models.AdminLoginLog
	var loginAtStr string
	err := r.db.QueryRow(query).Scan(
		&log.ID,
		&log.Username,
		&log.IPAddress,
		&log.UserAgent,
		&log.Location,
		&log.DeviceInfo,
		&loginAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last admin login: %w", err)
	}

	if t, err := time.Parse("2006-01-02 15:04:05", loginAtStr); err == nil {
		log.LoginAt = t
	} else if t, err := time.Parse(time.RFC3339, loginAtStr); err == nil {
		log.LoginAt = t
	}

	return &log, nil
}

// GetLoginHistory returns the last N admin login records
func (r *AdminLoginRepository) GetLoginHistory(limit int) ([]*models.AdminLoginLog, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT id, username, ip_address, user_agent, location, device_info, login_at
		FROM admin_login_logs
		ORDER BY id DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query admin login history: %w", err)
	}
	defer rows.Close()

	var logs []*models.AdminLoginLog
	for rows.Next() {
		var log models.AdminLoginLog
		var loginAtStr string
		err := rows.Scan(
			&log.ID,
			&log.Username,
			&log.IPAddress,
			&log.UserAgent,
			&log.Location,
			&log.DeviceInfo,
			&loginAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan admin login log: %w", err)
		}

		if t, err := time.Parse("2006-01-02 15:04:05", loginAtStr); err == nil {
			log.LoginAt = t
		} else if t, err := time.Parse(time.RFC3339, loginAtStr); err == nil {
			log.LoginAt = t
		}

		logs = append(logs, &log)
	}

	return logs, nil
}
