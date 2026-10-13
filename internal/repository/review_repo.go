package repository

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"storeku/internal/database"
	"storeku/internal/models"
)

type ReviewRepository struct {
	db *database.DB
}

func NewReviewRepository(db *database.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

// Create inserts a new product review
func (r *ReviewRepository) Create(rev *models.ProductReview) error {
	rev.ProductSlug = strings.TrimSpace(rev.ProductSlug)
	if rev.ProductSlug == "" {
		return fmt.Errorf("product slug wajib diisi")
	}

	rev.AuthorName = strings.TrimSpace(rev.AuthorName)
	if rev.AuthorName == "" {
		rev.AuthorName = "Pelanggan Terverifikasi"
	}

	if rev.Rating < 1 {
		rev.Rating = 1
	}
	if rev.Rating > 5 {
		rev.Rating = 5
	}

	rev.ReviewText = strings.TrimSpace(rev.ReviewText)

	isVisibleInt := 0
	if rev.IsVisible {
		isVisibleInt = 1
	}

	query := `
		INSERT INTO product_reviews (
			product_slug, order_invoice, author_name, rating, review_text, is_visible, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	res, err := r.db.Exec(query, rev.ProductSlug, strings.TrimSpace(rev.OrderInvoice), rev.AuthorName, rev.Rating, rev.ReviewText, isVisibleInt)
	if err != nil {
		return fmt.Errorf("gagal menyimpan ulasan: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		rev.ID = id
	}
	return nil
}

// GetByID finds a single review by ID
func (r *ReviewRepository) GetByID(id int64) (*models.ProductReview, error) {
	query := `
		SELECT id, product_slug, order_invoice, author_name, rating, review_text, is_visible, created_at, updated_at
		FROM product_reviews
		WHERE id = ?
	`
	var rev models.ProductReview
	var isVisibleInt int
	err := r.db.QueryRow(query, id).Scan(
		&rev.ID,
		&rev.ProductSlug,
		&rev.OrderInvoice,
		&rev.AuthorName,
		&rev.Rating,
		&rev.ReviewText,
		&isVisibleInt,
		&rev.CreatedAt,
		&rev.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil ulasan: %w", err)
	}
	rev.IsVisible = isVisibleInt == 1
	return &rev, nil
}

// Update modifies an existing review
func (r *ReviewRepository) Update(rev *models.ProductReview) error {
	if rev.Rating < 1 {
		rev.Rating = 1
	}
	if rev.Rating > 5 {
		rev.Rating = 5
	}

	isVisibleInt := 0
	if rev.IsVisible {
		isVisibleInt = 1
	}

	query := `
		UPDATE product_reviews SET
			author_name = ?, rating = ?, review_text = ?, is_visible = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	res, err := r.db.Exec(query, strings.TrimSpace(rev.AuthorName), rev.Rating, strings.TrimSpace(rev.ReviewText), isVisibleInt, rev.ID)
	if err != nil {
		return fmt.Errorf("gagal memperbarui ulasan: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("ulasan tidak ditemukan")
	}
	return nil
}

// Delete removes a review
func (r *ReviewRepository) Delete(id int64) error {
	res, err := r.db.Exec("DELETE FROM product_reviews WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("gagal menghapus ulasan: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("ulasan tidak ditemukan")
	}
	return nil
}

// HasOrderReviewed checks if a review already exists for this order invoice
func (r *ReviewRepository) HasOrderReviewed(orderInvoice string) (bool, error) {
	invoice := strings.TrimSpace(orderInvoice)
	if invoice == "" {
		return false, nil
	}
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM product_reviews WHERE order_invoice = ?", invoice).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetByProductSlug returns reviews for a product with pagination
func (r *ReviewRepository) GetByProductSlug(slug string, visibleOnly bool, limit int, offset int) ([]*models.ProductReview, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, product_slug, order_invoice, author_name, rating, review_text, is_visible, created_at, updated_at
		FROM product_reviews
		WHERE product_slug = ?
	`
	if visibleOnly {
		query += " AND is_visible = 1"
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"

	rows, err := r.db.Query(query, slug, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar ulasan: %w", err)
	}
	defer rows.Close()

	var reviews []*models.ProductReview
	for rows.Next() {
		var rev models.ProductReview
		var isVisibleInt int
		err := rows.Scan(
			&rev.ID,
			&rev.ProductSlug,
			&rev.OrderInvoice,
			&rev.AuthorName,
			&rev.Rating,
			&rev.ReviewText,
			&isVisibleInt,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		rev.IsVisible = isVisibleInt == 1
		reviews = append(reviews, &rev)
	}
	if reviews == nil {
		reviews = []*models.ProductReview{}
	}
	return reviews, nil
}

// GetAll returns all reviews for admin moderation
func (r *ReviewRepository) GetAll(limit int, offset int) ([]*models.ProductReview, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT id, product_slug, order_invoice, author_name, rating, review_text, is_visible, created_at, updated_at
		FROM product_reviews
		ORDER BY id DESC LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil seluruh ulasan: %w", err)
	}
	defer rows.Close()

	var reviews []*models.ProductReview
	for rows.Next() {
		var rev models.ProductReview
		var isVisibleInt int
		err := rows.Scan(
			&rev.ID,
			&rev.ProductSlug,
			&rev.OrderInvoice,
			&rev.AuthorName,
			&rev.Rating,
			&rev.ReviewText,
			&isVisibleInt,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		rev.IsVisible = isVisibleInt == 1
		reviews = append(reviews, &rev)
	}
	if reviews == nil {
		reviews = []*models.ProductReview{}
	}
	return reviews, nil
}

// GetStatsByProductSlug calculates average rating, total reviews, and star breakdown
func (r *ReviewRepository) GetStatsByProductSlug(slug string) (*models.ReviewStats, error) {
	stats := &models.ReviewStats{
		Distribution: map[int]int{
			1: 0,
			2: 0,
			3: 0,
			4: 0,
			5: 0,
		},
	}

	query := `
		SELECT rating, COUNT(*)
		FROM product_reviews
		WHERE product_slug = ? AND is_visible = 1
		GROUP BY rating
	`
	rows, err := r.db.Query(query, slug)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung statistik ulasan: %w", err)
	}
	defer rows.Close()

	totalReviews := 0
	sumStars := 0
	for rows.Next() {
		var star, count int
		if err := rows.Scan(&star, &count); err != nil {
			return nil, err
		}
		stats.Distribution[star] = count
		totalReviews += count
		sumStars += (star * count)
	}

	stats.TotalReviews = totalReviews
	if totalReviews > 0 {
		avg := float64(sumStars) / float64(totalReviews)
		stats.AverageRating = math.Round(avg*10.0) / 10.0
	} else {
		stats.AverageRating = 5.0 // Default 5.0 if no reviews yet
	}

	return stats, nil
}

// GetAllProductStats returns average rating and review count for all products in bulk
func (r *ReviewRepository) GetAllProductStats() (map[string]*models.ReviewStats, error) {
	query := `
		SELECT product_slug, AVG(rating) as avg_rating, COUNT(*) as cnt
		FROM product_reviews
		WHERE is_visible = 1
		GROUP BY product_slug
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung agregasi statistik ulasan: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.ReviewStats)
	for rows.Next() {
		var slug string
		var avg float64
		var cnt int
		if err := rows.Scan(&slug, &avg, &cnt); err != nil {
			return nil, err
		}
		result[slug] = &models.ReviewStats{
			AverageRating: math.Round(avg*10.0) / 10.0,
			TotalReviews:  cnt,
		}
	}
	return result, nil
}
