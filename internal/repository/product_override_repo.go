package repository

import (
	"database/sql"
	"fmt"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
)

type ProductOverrideRepository struct {
	db *database.DB
}

func NewProductOverrideRepository(db *database.DB) *ProductOverrideRepository {
	return &ProductOverrideRepository{db: db}
}

// GetAll returns all product overrides mapped by product_slug
func (r *ProductOverrideRepository) GetAll() (map[string]*models.ProductOverride, error) {
	query := `
		SELECT product_slug, custom_name, COALESCE(custom_type, ''), COALESCE(custom_region, ''), COALESCE(custom_warranty, ''), COALESCE(custom_stock, -1),
		       custom_price_idr, promo_price_idr, custom_description, custom_terms, is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
		FROM product_overrides
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query product overrides: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.ProductOverride)
	for rows.Next() {
		var o models.ProductOverride
		var isActiveInt, isForceOutOfStockInt, isAutoSuspendedInt int
		err := rows.Scan(
			&o.ProductSlug,
			&o.CustomName,
			&o.CustomType,
			&o.CustomRegion,
			&o.CustomWarranty,
			&o.CustomStock,
			&o.CustomPriceIDR,
			&o.PromoPriceIDR,
			&o.CustomDescription,
			&o.CustomTerms,
			&isActiveInt,
			&isForceOutOfStockInt,
			&o.BaselineModalIDR,
			&isAutoSuspendedInt,
			&o.CreatedAt,
			&o.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product override: %w", err)
		}
		o.IsActive = (isActiveInt == 1)
		o.IsForceOutOfStock = (isForceOutOfStockInt == 1)
		o.IsAutoSuspended = (isAutoSuspendedInt == 1)
		result[o.ProductSlug] = &o
	}

	return result, nil
}

// GetBySlug retrieves an override for a single product slug
func (r *ProductOverrideRepository) GetBySlug(slug string) (*models.ProductOverride, error) {
	query := `
		SELECT product_slug, custom_name, COALESCE(custom_type, ''), COALESCE(custom_region, ''), COALESCE(custom_warranty, ''), COALESCE(custom_stock, -1),
		       custom_price_idr, promo_price_idr, custom_description, custom_terms, is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
		FROM product_overrides
		WHERE product_slug = ?
		LIMIT 1
	`
	var o models.ProductOverride
	var isActiveInt, isForceOutOfStockInt, isAutoSuspendedInt int
	err := r.db.QueryRow(query, slug).Scan(
		&o.ProductSlug,
		&o.CustomName,
		&o.CustomType,
		&o.CustomRegion,
		&o.CustomWarranty,
		&o.CustomStock,
		&o.CustomPriceIDR,
		&o.PromoPriceIDR,
		&o.CustomDescription,
		&o.CustomTerms,
		&isActiveInt,
		&isForceOutOfStockInt,
		&o.BaselineModalIDR,
		&isAutoSuspendedInt,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product override: %w", err)
	}
	o.IsActive = (isActiveInt == 1)
	o.IsForceOutOfStock = (isForceOutOfStockInt == 1)
	o.IsAutoSuspended = (isAutoSuspendedInt == 1)
	return &o, nil
}

// Upsert saves or updates a product override
func (r *ProductOverrideRepository) Upsert(o *models.ProductOverride) error {
	query := `
		INSERT INTO product_overrides (
			product_slug, custom_name, custom_type, custom_region, custom_warranty, custom_stock,
			custom_price_idr, promo_price_idr, custom_description, custom_terms,
			is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(product_slug) DO UPDATE SET
			custom_name = excluded.custom_name,
			custom_type = excluded.custom_type,
			custom_region = excluded.custom_region,
			custom_warranty = excluded.custom_warranty,
			custom_stock = excluded.custom_stock,
			custom_price_idr = excluded.custom_price_idr,
			promo_price_idr = excluded.promo_price_idr,
			custom_description = excluded.custom_description,
			custom_terms = excluded.custom_terms,
			is_active = excluded.is_active,
			is_force_out_of_stock = excluded.is_force_out_of_stock,
			baseline_modal_idr = excluded.baseline_modal_idr,
			is_auto_suspended = excluded.is_auto_suspended,
			updated_at = CURRENT_TIMESTAMP
	`
	isActiveInt := 0
	if o.IsActive {
		isActiveInt = 1
	}
	isForceOutOfStockInt := 0
	if o.IsForceOutOfStock {
		isForceOutOfStockInt = 1
	}
	isAutoSuspendedInt := 0
	if o.IsAutoSuspended {
		isAutoSuspendedInt = 1
	}

	_, err := r.db.Exec(query,
		o.ProductSlug,
		o.CustomName,
		o.CustomType,
		o.CustomRegion,
		o.CustomWarranty,
		o.CustomStock,
		o.CustomPriceIDR,
		o.PromoPriceIDR,
		o.CustomDescription,
		o.CustomTerms,
		isActiveInt,
		isForceOutOfStockInt,
		o.BaselineModalIDR,
		isAutoSuspendedInt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert product override: %w", err)
	}
	o.UpdatedAt = time.Now()
	return nil
}

// Delete removes an override, reverting the product to DigitalCore defaults
func (r *ProductOverrideRepository) Delete(slug string) error {
	query := `DELETE FROM product_overrides WHERE product_slug = ?`
	_, err := r.db.Exec(query, slug)
	if err != nil {
		return fmt.Errorf("failed to delete product override: %w", err)
	}
	return nil
}
