package repository

import (
	"database/sql"
	"fmt"
	"strings"
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

// RecordLogin inserts a new login event (either SUCCESS or FAILED) into admin_login_logs
func (r *AdminLoginRepository) RecordLogin(username, ipAddress, userAgent, location, deviceInfo, status, failureReason string) error {
	if status == "" {
		status = "SUCCESS"
	}
	query := `
		INSERT INTO admin_login_logs (username, ip_address, user_agent, location, device_info, status, failure_reason, login_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := r.db.Exec(query, username, ipAddress, userAgent, location, deviceInfo, status, failureReason)
	if err != nil {
		return fmt.Errorf("failed to record admin login: %w", err)
	}
	return nil
}

// GetLastLogin returns the most recent admin login record
func (r *AdminLoginRepository) GetLastLogin() (*models.AdminLoginLog, error) {
	query := `
		SELECT id, username, ip_address, user_agent, location, device_info, COALESCE(status, 'SUCCESS'), COALESCE(failure_reason, ''), login_at
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
		&log.Status,
		&log.FailureReason,
		&loginAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last admin login: %w", err)
	}

	log.LoginAt = parseSQLiteTime(loginAtStr)
	return &log, nil
}

// GetLoginHistory returns the last N admin login records
func (r *AdminLoginRepository) GetLoginHistory(limit int) ([]*models.AdminLoginLog, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT id, username, ip_address, user_agent, location, device_info, COALESCE(status, 'SUCCESS'), COALESCE(failure_reason, ''), login_at
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
			&log.Status,
			&log.FailureReason,
			&loginAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan admin login log: %w", err)
		}
		log.LoginAt = parseSQLiteTime(loginAtStr)
		logs = append(logs, &log)
	}

	return logs, nil
}

// GetSecurityLogs returns filtered logs with total count for the dedicated security page
func (r *AdminLoginRepository) GetSecurityLogs(statusFilter, searchQuery string, limit, offset int) ([]*models.AdminLoginLog, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}

	statusFilter = strings.ToUpper(strings.TrimSpace(statusFilter))
	if statusFilter == "SUCCESS" || statusFilter == "FAILED" {
		conditions = append(conditions, "status = ?")
		args = append(args, statusFilter)
	}

	searchQuery = strings.TrimSpace(searchQuery)
	if searchQuery != "" {
		likeTerm := "%" + searchQuery + "%"
		conditions = append(conditions, "(username LIKE ? OR ip_address LIKE ? OR location LIKE ?)")
		args = append(args, likeTerm, likeTerm, likeTerm)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 1. Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM admin_login_logs %s", whereClause)
	var total int64
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count security logs: %w", err)
	}

	// 2. Query page items
	dataQuery := fmt.Sprintf(`
		SELECT id, username, ip_address, user_agent, location, device_info, COALESCE(status, 'SUCCESS'), COALESCE(failure_reason, ''), login_at
		FROM admin_login_logs
		%s
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query security logs: %w", err)
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
			&log.Status,
			&log.FailureReason,
			&loginAtStr,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan security log: %w", err)
		}
		log.LoginAt = parseSQLiteTime(loginAtStr)
		logs = append(logs, &log)
	}

	return logs, total, nil
}

// GetSecurityStats computes summary metrics (total success, total failed, and last failed attacker)
func (r *AdminLoginRepository) GetSecurityStats() (*models.SecurityStats, error) {
	stats := &models.SecurityStats{}

	// Total Success
	_ = r.db.QueryRow("SELECT COUNT(*) FROM admin_login_logs WHERE status = 'SUCCESS'").Scan(&stats.TotalSuccess)

	// Total Failed
	_ = r.db.QueryRow("SELECT COUNT(*) FROM admin_login_logs WHERE status = 'FAILED'").Scan(&stats.TotalFailed)

	// Last Failed Attempt
	var lastFailedIP, lastFailedUser, lastFailedTimeStr string
	err := r.db.QueryRow(`
		SELECT ip_address, username, login_at
		FROM admin_login_logs
		WHERE status = 'FAILED'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&lastFailedIP, &lastFailedUser, &lastFailedTimeStr)
	if err == nil {
		stats.LastFailedIP = lastFailedIP
		stats.LastFailedUser = lastFailedUser
		t := parseSQLiteTime(lastFailedTimeStr)
		stats.LastFailedAt = &t
	}

	return stats, nil
}

func parseSQLiteTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
}
