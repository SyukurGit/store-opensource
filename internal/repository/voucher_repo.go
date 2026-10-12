package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
)

type VoucherRepository struct {
	db *database.DB
}

func NewVoucherRepository(db *database.DB) *VoucherRepository {
	return &VoucherRepository{db: db}
}

// Create inserts a newly created and signed voucher
func (r *VoucherRepository) Create(v *models.Voucher) error {
	v.Code = strings.ToUpper(strings.TrimSpace(v.Code))
	var expiresAtUTC *time.Time
	if v.ExpiresAt != nil {
		t := v.ExpiresAt.UTC()
		expiresAtUTC = &t
	}

	query := `
		INSERT INTO vouchers (
			code, discount_idr, min_spend_idr, max_usage, used_count,
			expires_at, is_active, signature, note, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`
	res, err := r.db.Exec(
		query,
		v.Code, v.DiscountIDR, v.MinSpendIDR, v.MaxUsage, v.UsedCount,
		expiresAtUTC, v.IsActive, v.Signature, v.Note,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("kode voucher '%s' sudah pernah dibuat sebelumnya", v.Code)
		}
		return fmt.Errorf("gagal membuat voucher: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		v.ID = id
	}
	return nil
}

// GetByCode retrieves a voucher by its uppercase code
func (r *VoucherRepository) GetByCode(code string) (*models.Voucher, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	query := `
		SELECT
			id, code, discount_idr, min_spend_idr, max_usage, used_count,
			expires_at, is_active, signature, note, created_at, updated_at
		FROM vouchers
		WHERE code = ?
	`
	row := r.db.QueryRow(query, code)
	return scanVoucher(row)
}

// GetByID retrieves a voucher by its ID
func (r *VoucherRepository) GetByID(id int64) (*models.Voucher, error) {
	query := `
		SELECT
			id, code, discount_idr, min_spend_idr, max_usage, used_count,
			expires_at, is_active, signature, note, created_at, updated_at
		FROM vouchers
		WHERE id = ?
	`
	row := r.db.QueryRow(query, id)
	return scanVoucher(row)
}

// GetAll returns all vouchers ordered by newest first
func (r *VoucherRepository) GetAll() ([]*models.Voucher, error) {
	query := `
		SELECT
			id, code, discount_idr, min_spend_idr, max_usage, used_count,
			expires_at, is_active, signature, note, created_at, updated_at
		FROM vouchers
		ORDER BY id DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar voucher: %w", err)
	}
	defer rows.Close()

	var list []*models.Voucher
	for rows.Next() {
		v, err := scanVoucherRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, nil
}

// ToggleActive toggles is_active status of a voucher
func (r *VoucherRepository) ToggleActive(id int64) error {
	query := `
		UPDATE vouchers
		SET is_active = CASE WHEN is_active = 1 THEN 0 ELSE 1 END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	res, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("gagal mengubah status voucher: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("voucher dengan ID %d tidak ditemukan", id)
	}
	return nil
}

// Delete removes a voucher by ID
func (r *VoucherRepository) Delete(id int64) error {
	query := "DELETE FROM vouchers WHERE id = ?"
	res, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus voucher: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("voucher dengan ID %d tidak ditemukan", id)
	}
	return nil
}

// ConsumeVoucherAtomic atomically increments used_count if voucher is active and quota remains
func (r *VoucherRepository) ConsumeVoucherAtomic(code string) (*models.Voucher, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	nowUTC := time.Now().UTC()

	query := `
		UPDATE vouchers
		SET used_count = used_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE code = ?
		  AND is_active = 1
		  AND used_count < max_usage
		  AND (expires_at IS NULL OR expires_at > ?)
	`
	res, err := r.db.Exec(query, code, nowUTC)
	if err != nil {
		return nil, fmt.Errorf("gagal memproses penggunaan voucher: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		// Differentiate why it failed by inspecting the voucher
		v, errCheck := r.GetByCode(code)
		if errCheck != nil {
			return nil, fmt.Errorf("kode voucher '%s' tidak ditemukan", code)
		}
		if !v.IsActive {
			return nil, fmt.Errorf("voucher '%s' sedang dinonaktifkan", code)
		}
		if v.ExpiresAt != nil && v.ExpiresAt.Before(nowUTC) {
			return nil, fmt.Errorf("voucher '%s' sudah melewati batas tanggal kedaluwarsa", code)
		}
		if v.UsedCount >= v.MaxUsage {
			return nil, fmt.Errorf("kuota pemakaian voucher '%s' sudah habis terpakai", code)
		}
		return nil, fmt.Errorf("voucher '%s' tidak dapat digunakan saat ini", code)
	}

	return r.GetByCode(code)
}

// RevertVoucherUsage decrements used_count (used only if QRIS creation at payment gateway fails)
func (r *VoucherRepository) RevertVoucherUsage(code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	query := `
		UPDATE vouchers
		SET used_count = CASE WHEN used_count > 0 THEN used_count - 1 ELSE 0 END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE code = ?
	`
	_, err := r.db.Exec(query, code)
	return err
}

func scanVoucher(row *sql.Row) (*models.Voucher, error) {
	v := &models.Voucher{}
	err := row.Scan(
		&v.ID, &v.Code, &v.DiscountIDR, &v.MinSpendIDR, &v.MaxUsage, &v.UsedCount,
		&v.ExpiresAt, &v.IsActive, &v.Signature, &v.Note, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func scanVoucherRows(rows *sql.Rows) (*models.Voucher, error) {
	v := &models.Voucher{}
	err := rows.Scan(
		&v.ID, &v.Code, &v.DiscountIDR, &v.MinSpendIDR, &v.MaxUsage, &v.UsedCount,
		&v.ExpiresAt, &v.IsActive, &v.Signature, &v.Note, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}
