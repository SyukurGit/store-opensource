package repository

import (
	"fmt"
	"os"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
)

type MaintenanceRepository struct {
	db *database.DB
}

func NewMaintenanceRepository(db *database.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// formatBytes converts byte count to human-readable string (KB, MB, GB)
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// getDatabaseFileSize returns total size of sqlite db plus WAL/SHM files
func getDatabaseFileSize(dbPath string) int64 {
	var total int64
	files := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			total += fi.Size()
		}
	}
	return total
}

// GetStats returns current counts and database metrics
func (r *MaintenanceRepository) GetStats(dbPath string) (*models.DatabaseStats, error) {
	stats := &models.DatabaseStats{}

	sizeBytes := getDatabaseFileSize(dbPath)
	stats.DatabaseSizeBytes = sizeBytes
	stats.DatabaseSizeFormatted = formatBytes(sizeBytes)

	// Orders counts
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&stats.TotalOrders)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE payment_status = 'PAID'").Scan(&stats.PaidOrders)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE payment_status = 'PENDING'").Scan(&stats.PendingOrders)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE payment_status IN ('EXPIRED', 'CANCELLED')").Scan(&stats.ExpiredCancelledOrders)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE created_at < datetime('now', '-3 days')").Scan(&stats.OrdersOlderThan3Days)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE created_at < datetime('now', '-7 days')").Scan(&stats.OrdersOlderThan7Days)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE created_at < datetime('now', '-30 days')").Scan(&stats.OrdersOlderThan30Days)

	// Orders with serial keys delivered older than 3 days
	_ = r.db.QueryRow(`
		SELECT COUNT(*) FROM orders 
		WHERE payment_status = 'PAID' 
		  AND delivered_items != '[]' 
		  AND delivered_items != '' 
		  AND created_at < datetime('now', '-3 days')
	`).Scan(&stats.OrdersWithKeysOlder3Days)

	// Login logs counts
	_ = r.db.QueryRow("SELECT COUNT(*) FROM admin_login_logs").Scan(&stats.TotalLoginLogs)
	_ = r.db.QueryRow("SELECT COUNT(*) FROM admin_login_logs WHERE status != 'SUCCESS'").Scan(&stats.FailedLoginLogs)

	// Vouchers counts
	_ = r.db.QueryRow("SELECT COUNT(*) FROM vouchers").Scan(&stats.TotalVouchers)
	_ = r.db.QueryRow(`
		SELECT COUNT(*) FROM vouchers 
		WHERE (expires_at IS NOT NULL AND expires_at < datetime('now')) 
		   OR (max_usage > 0 AND used_count >= max_usage) 
		   OR is_active = 0
	`).Scan(&stats.ExpiredUsedVouchers)

	// Cached products count
	_ = r.db.QueryRow("SELECT COUNT(*) FROM cached_products").Scan(&stats.CachedProductsCount)

	return stats, nil
}

// CleanDatabase executes selective cleanup operations and optional VACUUM
func (r *MaintenanceRepository) CleanDatabase(dbPath string, req *models.DatabaseCleanRequest) (*models.DatabaseCleanResult, error) {
	sizeBefore := getDatabaseFileSize(dbPath)
	res := &models.DatabaseCleanResult{
		DatabaseSizeBefore: formatBytes(sizeBefore),
	}

	// 1. Clean Orders
	if req.CleanOrders {
		var query string
		var args []interface{}

		switch req.OrdersMode {
		case "ALL":
			query = "DELETE FROM orders"
		case "EXPIRED_CANCELLED":
			query = "DELETE FROM orders WHERE payment_status IN ('EXPIRED', 'CANCELLED')"
		case "OLDER_THAN":
			days := req.OrdersOlderDays
			if days <= 0 {
				days = 3
			}
			query = "DELETE FROM orders WHERE created_at < datetime('now', ?)"
			args = append(args, fmt.Sprintf("-%d days", days))
		default:
			// Default to expired/cancelled if unspecified
			query = "DELETE FROM orders WHERE payment_status IN ('EXPIRED', 'CANCELLED')"
		}

		execRes, err := r.db.Exec(query, args...)
		if err != nil {
			return nil, fmt.Errorf("gagal menghapus pesanan: %w", err)
		}
		rows, _ := execRes.RowsAffected()
		res.OrdersDeleted = rows
	}

	// 2. Clean Delivered Keys (Reset delivered_items to '[]' on older paid orders)
	if req.CleanDeliveredKeys {
		days := req.KeysOlderDays
		if days <= 0 {
			days = 3
		}
		query := `
			UPDATE orders 
			SET delivered_items = '[]', updated_at = CURRENT_TIMESTAMP 
			WHERE payment_status = 'PAID' 
			  AND delivered_items != '[]' 
			  AND delivered_items != '' 
			  AND created_at < datetime('now', ?)
		`
		execRes, err := r.db.Exec(query, fmt.Sprintf("-%d days", days))
		if err != nil {
			return nil, fmt.Errorf("gagal mengosongkan serial key lama: %w", err)
		}
		rows, _ := execRes.RowsAffected()
		res.KeysCleaned = rows
	}

	// 3. Clean Login Logs
	if req.CleanLoginLogs {
		var query string
		switch req.LogsMode {
		case "ALL":
			query = "DELETE FROM admin_login_logs"
		case "FAILED_ONLY":
			query = "DELETE FROM admin_login_logs WHERE status != 'SUCCESS'"
		case "OLDER_THAN_7_DAYS":
			query = "DELETE FROM admin_login_logs WHERE login_at < datetime('now', '-7 days')"
		case "OLDER_THAN_30_DAYS":
			query = "DELETE FROM admin_login_logs WHERE login_at < datetime('now', '-30 days')"
		default:
			query = "DELETE FROM admin_login_logs WHERE login_at < datetime('now', '-7 days')"
		}

		execRes, err := r.db.Exec(query)
		if err != nil {
			return nil, fmt.Errorf("gagal menghapus log login: %w", err)
		}
		rows, _ := execRes.RowsAffected()
		res.LogsDeleted = rows
	}

	// 4. Clean Vouchers
	if req.CleanVouchers {
		var query string
		switch req.VouchersMode {
		case "ALL":
			query = "DELETE FROM vouchers"
		case "EXPIRED_USED":
			query = `
				DELETE FROM vouchers 
				WHERE (expires_at IS NOT NULL AND expires_at < datetime('now')) 
				   OR (max_usage > 0 AND used_count >= max_usage) 
				   OR is_active = 0
			`
		default:
			query = `
				DELETE FROM vouchers 
				WHERE (expires_at IS NOT NULL AND expires_at < datetime('now')) 
				   OR (max_usage > 0 AND used_count >= max_usage) 
				   OR is_active = 0
			`
		}

		execRes, err := r.db.Exec(query)
		if err != nil {
			return nil, fmt.Errorf("gagal membersihkan voucher: %w", err)
		}
		rows, _ := execRes.RowsAffected()
		res.VouchersDeleted = rows
	}

	// 5. Clean Cached Products
	if req.CleanCachedProducts {
		execRes, err := r.db.Exec("DELETE FROM cached_products")
		if err != nil {
			return nil, fmt.Errorf("gagal mengosongkan cache produk: %w", err)
		}
		rows, _ := execRes.RowsAffected()
		res.CachedCleaned = rows
	}

	// 6. SQLite VACUUM
	if req.RunVacuum {
		// VACUUM defragments the database and shrinks the file size on disk
		if _, err := r.db.Exec("VACUUM;"); err != nil {
			// Log vacuum error without failing the whole cleanup
			res.VacuumExecuted = false
		} else {
			res.VacuumExecuted = true
		}
	}

	// Measure size after
	time.Sleep(50 * time.Millisecond) // brief pause for filesystem sync
	sizeAfter := getDatabaseFileSize(dbPath)
	res.DatabaseSizeAfter = formatBytes(sizeAfter)
	res.Message = fmt.Sprintf("Pembersihan database selesai. Ukuran: %s ➔ %s", res.DatabaseSizeBefore, res.DatabaseSizeAfter)

	return res, nil
}
