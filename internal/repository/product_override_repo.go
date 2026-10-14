package repository

import (
	"database/sql"
	"fmt"
	"strings"
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
		       custom_price_idr, promo_price_idr, custom_description, custom_terms,
		       COALESCE(custom_name_en, ''), COALESCE(custom_type_en, ''), COALESCE(custom_region_en, ''), COALESCE(custom_warranty_en, ''), COALESCE(custom_description_en, ''), COALESCE(custom_terms_en, ''),
		       COALESCE(display_order, 0), COALESCE(sales_count, 10),
		       is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
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
			&o.CustomNameEN,
			&o.CustomTypeEN,
			&o.CustomRegionEN,
			&o.CustomWarrantyEN,
			&o.CustomDescriptionEN,
			&o.CustomTermsEN,
			&o.DisplayOrder,
			&o.SalesCount,
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
		if o.SalesCount <= 0 {
			o.SalesCount = 10
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
		       custom_price_idr, promo_price_idr, custom_description, custom_terms,
		       COALESCE(custom_name_en, ''), COALESCE(custom_type_en, ''), COALESCE(custom_region_en, ''), COALESCE(custom_warranty_en, ''), COALESCE(custom_description_en, ''), COALESCE(custom_terms_en, ''),
		       COALESCE(display_order, 0), COALESCE(sales_count, 10),
		       is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
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
		&o.CustomNameEN,
		&o.CustomTypeEN,
		&o.CustomRegionEN,
		&o.CustomWarrantyEN,
		&o.CustomDescriptionEN,
		&o.CustomTermsEN,
		&o.DisplayOrder,
		&o.SalesCount,
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
	if o.SalesCount <= 0 {
		o.SalesCount = 10
	}
	o.IsActive = (isActiveInt == 1)
	o.IsForceOutOfStock = (isForceOutOfStockInt == 1)
	o.IsAutoSuspended = (isAutoSuspendedInt == 1)
	return &o, nil
}

// Upsert saves or updates a product override
func (r *ProductOverrideRepository) Upsert(o *models.ProductOverride) error {
	salesCount := o.SalesCount
	if salesCount <= 0 {
		salesCount = 10
	}
	query := `
		INSERT INTO product_overrides (
			product_slug, custom_name, custom_type, custom_region, custom_warranty, custom_stock,
			custom_price_idr, promo_price_idr, custom_description, custom_terms,
			display_order, sales_count,
			is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
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
			display_order = excluded.display_order,
			sales_count = excluded.sales_count,
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
		o.DisplayOrder,
		salesCount,
		isActiveInt,
		isForceOutOfStockInt,
		o.BaselineModalIDR,
		isAutoSuspendedInt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert product override: %w", err)
	}
	o.SalesCount = salesCount
	o.UpdatedAt = time.Now()
	return nil
}

// UpdateDisplayStats updates or initializes display_order and sales_count for an upstream product
func (r *ProductOverrideRepository) UpdateDisplayStats(slug string, displayOrder int, salesCount int) error {
	if salesCount <= 0 {
		salesCount = 10
	}
	query := `
		INSERT INTO product_overrides (
			product_slug, custom_name, custom_stock, display_order, sales_count,
			is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
		) VALUES (?, '', -1, ?, ?, 1, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(product_slug) DO UPDATE SET
			display_order = excluded.display_order,
			sales_count = excluded.sales_count,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.db.Exec(query, slug, displayOrder, salesCount)
	return err
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

// UpdateTranslation updates English translation fields for an upstream product override
func (r *ProductOverrideRepository) UpdateTranslation(slug string, req *models.UpdateTranslationRequest) error {
	query := `
		INSERT INTO product_overrides (
			product_slug, custom_name, custom_name_en, custom_type_en, custom_region_en,
			custom_warranty_en, custom_description_en, custom_terms_en, custom_stock,
			is_active, is_force_out_of_stock, baseline_modal_idr, is_auto_suspended, created_at, updated_at
		) VALUES (?, '', ?, ?, ?, ?, ?, ?, -1, 1, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(product_slug) DO UPDATE SET
			custom_name_en = excluded.custom_name_en,
			custom_type_en = excluded.custom_type_en,
			custom_region_en = excluded.custom_region_en,
			custom_warranty_en = excluded.custom_warranty_en,
			custom_description_en = excluded.custom_description_en,
			custom_terms_en = excluded.custom_terms_en,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.db.Exec(query,
		slug,
		strings.TrimSpace(req.NameEN),
		strings.TrimSpace(req.TypeEN),
		strings.TrimSpace(req.RegionEN),
		strings.TrimSpace(req.WarrantyEN),
		strings.TrimSpace(req.DescriptionEN),
		strings.TrimSpace(req.TermsEN),
	)
	return err
}

