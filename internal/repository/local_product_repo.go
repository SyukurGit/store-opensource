package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"storeku/internal/database"
	"storeku/internal/models"
)

type LocalProductRepository struct {
	db *database.DB
	mu sync.Mutex // Mutex for concurrency safety during key claiming
}

func NewLocalProductRepository(db *database.DB) *LocalProductRepository {
	return &LocalProductRepository{db: db}
}

// sanitizeKeys trims lines, removes duplicates/empty, and caps at maximum 10 keys
func sanitizeKeys(rawKeys []string) []string {
	var clean []string
	seen := make(map[string]bool)
	for _, k := range rawKeys {
		trimmed := strings.TrimSpace(k)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		clean = append(clean, trimmed)
		if len(clean) >= 10 {
			break // Max 10 keys enforced
		}
	}
	if clean == nil {
		clean = []string{}
	}
	return clean
}

func toLocalSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "_")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// Create inserts a new local product into the database
func (r *LocalProductRepository) Create(req *models.LocalProductRequest) (*models.LocalProduct, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, fmt.Errorf("nama produk wajib diisi")
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = toLocalSlug(req.Name)
	} else {
		slug = toLocalSlug(slug)
	}

	if slug == "" {
		return nil, fmt.Errorf("slug produk tidak valid")
	}

	// Check if slug is already used
	var existsID int64
	err := r.db.QueryRow("SELECT id FROM local_products WHERE slug = ?", slug).Scan(&existsID)
	if err == nil {
		return nil, fmt.Errorf("slug '%s' sudah digunakan oleh produk lain", slug)
	}

	if req.PriceIDR < 0 {
		req.PriceIDR = 0
	}

	refPrice := req.ReferencePriceIDR
	if refPrice <= 0 {
		refPrice = req.PriceIDR
	}

	cleanKeys := sanitizeKeys(req.StockKeys)
	keysBytes, _ := json.Marshal(cleanKeys)

	// Stock logic: if keys provided, stock = len(keys).
	// If 0 keys provided, stock is 99 (ready for manual admin delivery) unless force out of stock
	stock := len(cleanKeys)
	if len(cleanKeys) == 0 && !req.IsForceOutOfStock {
		stock = 99
	}

	discPercent := 0
	if req.HasPromo && req.PromoPriceIDR > 0 && refPrice > req.PromoPriceIDR {
		discPercent = int(float64(refPrice-req.PromoPriceIDR) / float64(refPrice) * 100.0)
	}

	prodType := strings.TrimSpace(req.Type)
	if prodType == "" {
		prodType = "Digital License"
	}

	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = "Global"
	}

	warranty := strings.TrimSpace(req.Warranty)
	if warranty == "" {
		warranty = "Full Garansi"
	}

	terms := strings.TrimSpace(req.Terms)
	if terms == "" {
		terms = "Membeli berarti menyetujui seluruh syarat & ketentuan toko."
	}

	isActiveInt := 0
	if req.IsActive {
		isActiveInt = 1
	}

	isForceOutInt := 0
	if req.IsForceOutOfStock {
		isForceOutInt = 1
		stock = 0
	}

	hasPromoInt := 0
	if req.HasPromo {
		hasPromoInt = 1
	}

	salesCount := req.SalesCount
	if salesCount <= 0 {
		salesCount = 10
	}

	query := `
		INSERT INTO local_products (
			slug, name, type, logo, region, warranty, description, terms,
			price_idr, reference_price_idr, promo_price_idr, has_promo, discount_percent,
			stock, stock_keys, display_order, sales_count, is_active, is_force_out_of_stock, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	res, err := r.db.Exec(query,
		slug, req.Name, prodType, strings.TrimSpace(req.Logo), region, warranty,
		req.Description, terms, req.PriceIDR, refPrice, req.PromoPriceIDR,
		hasPromoInt, discPercent, stock, string(keysBytes), req.DisplayOrder, salesCount, isActiveInt, isForceOutInt,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal menambahkan produk lokal: %w", err)
	}

	id, _ := res.LastInsertId()
	return r.GetByID(id)
}

// Update modifies an existing local product
func (r *LocalProductRepository) Update(id int64, req *models.LocalProductRequest) (*models.LocalProduct, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, fmt.Errorf("nama produk wajib diisi")
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = toLocalSlug(req.Name)
	} else {
		slug = toLocalSlug(slug)
	}

	// Verify slug not taken by another ID
	var existsID int64
	err := r.db.QueryRow("SELECT id FROM local_products WHERE slug = ? AND id != ?", slug, id).Scan(&existsID)
	if err == nil {
		return nil, fmt.Errorf("slug '%s' sudah digunakan oleh produk lain", slug)
	}

	refPrice := req.ReferencePriceIDR
	if refPrice <= 0 {
		refPrice = req.PriceIDR
	}

	cleanKeys := sanitizeKeys(req.StockKeys)
	keysBytes, _ := json.Marshal(cleanKeys)

	stock := len(cleanKeys)
	if len(cleanKeys) == 0 && !req.IsForceOutOfStock {
		stock = 99
	}

	discPercent := 0
	if req.HasPromo && req.PromoPriceIDR > 0 && refPrice > req.PromoPriceIDR {
		discPercent = int(float64(refPrice-req.PromoPriceIDR) / float64(refPrice) * 100.0)
	}

	prodType := strings.TrimSpace(req.Type)
	if prodType == "" {
		prodType = "Digital License"
	}

	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = "Global"
	}

	warranty := strings.TrimSpace(req.Warranty)
	if warranty == "" {
		warranty = "Full Garansi"
	}

	terms := strings.TrimSpace(req.Terms)
	if terms == "" {
		terms = "Membeli berarti menyetujui seluruh syarat & ketentuan toko."
	}

	isActiveInt := 0
	if req.IsActive {
		isActiveInt = 1
	}

	isForceOutInt := 0
	if req.IsForceOutOfStock {
		isForceOutInt = 1
		stock = 0
	}

	hasPromoInt := 0
	if req.HasPromo {
		hasPromoInt = 1
	}

	salesCount := req.SalesCount
	if salesCount <= 0 {
		salesCount = 10
	}

	query := `
		UPDATE local_products SET
			slug = ?, name = ?, type = ?, logo = ?, region = ?, warranty = ?, description = ?, terms = ?,
			price_idr = ?, reference_price_idr = ?, promo_price_idr = ?, has_promo = ?, discount_percent = ?,
			stock = ?, stock_keys = ?, display_order = ?, sales_count = ?, is_active = ?, is_force_out_of_stock = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err = r.db.Exec(query,
		slug, req.Name, prodType, strings.TrimSpace(req.Logo), region, warranty,
		req.Description, terms, req.PriceIDR, refPrice, req.PromoPriceIDR,
		hasPromoInt, discPercent, stock, string(keysBytes), req.DisplayOrder, salesCount, isActiveInt, isForceOutInt,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui produk lokal: %w", err)
	}

	return r.GetByID(id)
}

// GetByID finds a local product by primary key
func (r *LocalProductRepository) GetByID(id int64) (*models.LocalProduct, error) {
	query := `
		SELECT id, slug, name, type, logo, region, warranty, description, terms,
		       COALESCE(name_en, ''), COALESCE(type_en, ''), COALESCE(region_en, ''), COALESCE(warranty_en, ''), COALESCE(description_en, ''), COALESCE(terms_en, ''),
		       price_idr, reference_price_idr, promo_price_idr, has_promo, discount_percent,
		       stock, stock_keys, COALESCE(display_order, 0), COALESCE(sales_count, 10),
		       is_active, is_force_out_of_stock, created_at, updated_at
		FROM local_products
		WHERE id = ?
	`
	row := r.db.QueryRow(query, id)
	return r.scanProduct(row)
}

// GetBySlug finds a local product by its slug
func (r *LocalProductRepository) GetBySlug(slug string) (*models.LocalProduct, error) {
	query := `
		SELECT id, slug, name, type, logo, region, warranty, description, terms,
		       COALESCE(name_en, ''), COALESCE(type_en, ''), COALESCE(region_en, ''), COALESCE(warranty_en, ''), COALESCE(description_en, ''), COALESCE(terms_en, ''),
		       price_idr, reference_price_idr, promo_price_idr, has_promo, discount_percent,
		       stock, stock_keys, COALESCE(display_order, 0), COALESCE(sales_count, 10),
		       is_active, is_force_out_of_stock, created_at, updated_at
		FROM local_products
		WHERE slug = ?
	`
	row := r.db.QueryRow(query, slug)
	return r.scanProduct(row)
}

// GetAll returns all local products for admin view
func (r *LocalProductRepository) GetAll() ([]*models.LocalProduct, error) {
	query := `
		SELECT id, slug, name, type, logo, region, warranty, description, terms,
		       COALESCE(name_en, ''), COALESCE(type_en, ''), COALESCE(region_en, ''), COALESCE(warranty_en, ''), COALESCE(description_en, ''), COALESCE(terms_en, ''),
		       price_idr, reference_price_idr, promo_price_idr, has_promo, discount_percent,
		       stock, stock_keys, COALESCE(display_order, 0), COALESCE(sales_count, 10),
		       is_active, is_force_out_of_stock, created_at, updated_at
		FROM local_products
		ORDER BY id DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.LocalProduct
	for rows.Next() {
		p, err := r.scanProductFromRows(rows)
		if err != nil {
			continue
		}
		list = append(list, p)
	}
	return list, nil
}

// GetAllActive returns only active local products for the public store catalog
func (r *LocalProductRepository) GetAllActive() ([]*models.LocalProduct, error) {
	query := `
		SELECT id, slug, name, type, logo, region, warranty, description, terms,
		       COALESCE(name_en, ''), COALESCE(type_en, ''), COALESCE(region_en, ''), COALESCE(warranty_en, ''), COALESCE(description_en, ''), COALESCE(terms_en, ''),
		       price_idr, reference_price_idr, promo_price_idr, has_promo, discount_percent,
		       stock, stock_keys, COALESCE(display_order, 0), COALESCE(sales_count, 10),
		       is_active, is_force_out_of_stock, created_at, updated_at
		FROM local_products
		WHERE is_active = 1
		ORDER BY id DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.LocalProduct
	for rows.Next() {
		p, err := r.scanProductFromRows(rows)
		if err != nil {
			continue
		}
		list = append(list, p)
	}
	return list, nil
}

// UpdateDisplayStats updates display_order and sales_count for a local product
func (r *LocalProductRepository) UpdateDisplayStats(slug string, displayOrder int, salesCount int) error {
	if salesCount <= 0 {
		salesCount = 10
	}
	query := `UPDATE local_products SET display_order = ?, sales_count = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?`
	res, err := r.db.Exec(query, displayOrder, salesCount, slug)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("produk lokal '%s' tidak ditemukan", slug)
	}
	return nil
}

// Delete removes a local product by ID
func (r *LocalProductRepository) Delete(id int64) error {
	res, err := r.db.Exec("DELETE FROM local_products WHERE id = ?", id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("produk tidak ditemukan")
	}
	return nil
}

// ClaimKeys atomically takes 'qty' keys from stock_keys and reduces stock
// If keys are empty, it safely returns an empty slice without throwing error
func (r *LocalProductRepository) ClaimKeys(slug string, qty int) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var keysJSON string
	var currentStock int
	err := r.db.QueryRow("SELECT stock_keys, stock FROM local_products WHERE slug = ?", slug).Scan(&keysJSON, &currentStock)
	if err != nil {
		return nil, fmt.Errorf("produk lokal '%s' tidak ditemukan: %w", slug, err)
	}

	var keys []string
	_ = json.Unmarshal([]byte(keysJSON), &keys)

	if len(keys) == 0 {
		// Keys are empty. User requested: "Jika key kosong juga tidak apa-apa. Nanti user akan hubungi gue."
		return []string{}, nil
	}

	takeCount := qty
	if takeCount > len(keys) {
		takeCount = len(keys)
	}

	taken := keys[:takeCount]
	remaining := keys[takeCount:]

	remainingJSON, _ := json.Marshal(remaining)
	newStock := len(remaining)

	_, err = r.db.Exec("UPDATE local_products SET stock_keys = ?, stock = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?", string(remainingJSON), newStock, slug)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui sisa serial key: %w", err)
	}

	return taken, nil
}

// ReplenishKeys appends new serial keys up to max 10
func (r *LocalProductRepository) ReplenishKeys(id int64, newKeys []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var keysJSON string
	err := r.db.QueryRow("SELECT stock_keys FROM local_products WHERE id = ?", id).Scan(&keysJSON)
	if err != nil {
		return fmt.Errorf("produk lokal tidak ditemukan: %w", err)
	}

	var currentKeys []string
	_ = json.Unmarshal([]byte(keysJSON), &currentKeys)

	combined := append(currentKeys, newKeys...)
	clean := sanitizeKeys(combined)
	cleanJSON, _ := json.Marshal(clean)
	newStock := len(clean)

	_, err = r.db.Exec("UPDATE local_products SET stock_keys = ?, stock = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(cleanJSON), newStock, id)
	return err
}

func (r *LocalProductRepository) scanProduct(row *sql.Row) (*models.LocalProduct, error) {
	var p models.LocalProduct
	var keysJSON string
	var hasPromoInt, isActiveInt, isForceOutInt int

	err := row.Scan(
		&p.ID, &p.Slug, &p.Name, &p.Type, &p.Logo, &p.Region, &p.Warranty, &p.Description, &p.Terms,
		&p.NameEN, &p.TypeEN, &p.RegionEN, &p.WarrantyEN, &p.DescriptionEN, &p.TermsEN,
		&p.PriceIDR, &p.ReferencePriceIDR, &p.PromoPriceIDR, &hasPromoInt, &p.DiscountPercent,
		&p.Stock, &keysJSON, &p.DisplayOrder, &p.SalesCount, &isActiveInt, &isForceOutInt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if p.SalesCount <= 0 {
		p.SalesCount = 10
	}

	p.HasPromo = hasPromoInt == 1
	p.IsActive = isActiveInt == 1
	p.IsForceOutOfStock = isForceOutInt == 1

	_ = json.Unmarshal([]byte(keysJSON), &p.StockKeys)
	if p.StockKeys == nil {
		p.StockKeys = []string{}
	}

	return &p, nil
}

func (r *LocalProductRepository) scanProductFromRows(rows *sql.Rows) (*models.LocalProduct, error) {
	var p models.LocalProduct
	var keysJSON string
	var hasPromoInt, isActiveInt, isForceOutInt int

	err := rows.Scan(
		&p.ID, &p.Slug, &p.Name, &p.Type, &p.Logo, &p.Region, &p.Warranty, &p.Description, &p.Terms,
		&p.NameEN, &p.TypeEN, &p.RegionEN, &p.WarrantyEN, &p.DescriptionEN, &p.TermsEN,
		&p.PriceIDR, &p.ReferencePriceIDR, &p.PromoPriceIDR, &hasPromoInt, &p.DiscountPercent,
		&p.Stock, &keysJSON, &p.DisplayOrder, &p.SalesCount, &isActiveInt, &isForceOutInt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if p.SalesCount <= 0 {
		p.SalesCount = 10
	}

	p.HasPromo = hasPromoInt == 1
	p.IsActive = isActiveInt == 1
	p.IsForceOutOfStock = isForceOutInt == 1

	_ = json.Unmarshal([]byte(keysJSON), &p.StockKeys)
	if p.StockKeys == nil {
		p.StockKeys = []string{}
	}

	return &p, nil
}

// UpdateTranslation updates English textual fields for a local product
func (r *LocalProductRepository) UpdateTranslation(slug string, req *models.UpdateTranslationRequest) error {
	query := `
		UPDATE local_products SET
			name_en = ?, type_en = ?, region_en = ?, warranty_en = ?, description_en = ?, terms_en = ?, updated_at = CURRENT_TIMESTAMP
		WHERE slug = ?
	`
	res, err := r.db.Exec(query,
		strings.TrimSpace(req.NameEN),
		strings.TrimSpace(req.TypeEN),
		strings.TrimSpace(req.RegionEN),
		strings.TrimSpace(req.WarrantyEN),
		strings.TrimSpace(req.DescriptionEN),
		strings.TrimSpace(req.TermsEN),
		slug,
	)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("produk lokal '%s' tidak ditemukan", slug)
	}
	return nil
}
