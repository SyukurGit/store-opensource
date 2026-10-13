package repository

import (
	"path/filepath"
	"testing"

	"storeku/internal/database"
	"storeku/internal/models"
)

func TestReviewRepository(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_reviews.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	repo := NewReviewRepository(db)

	// 1. Create reviews
	r1 := &models.ProductReview{
		ProductSlug:  "canva_pro",
		OrderInvoice: "INV-20261013-0001",
		AuthorName:   "Budi Santoso",
		Rating:       5,
		ReviewText:   "Mantap sekali langsung aktif!",
		IsVisible:    true,
	}
	err = repo.Create(r1)
	if err != nil {
		t.Fatalf("failed to create review 1: %v", err)
	}
	if r1.ID == 0 {
		t.Errorf("expected review ID to be set")
	}

	r2 := &models.ProductReview{
		ProductSlug:  "canva_pro",
		OrderInvoice: "INV-20261013-0002",
		AuthorName:   "Siti Aisyah",
		Rating:       4,
		ReviewText:   "Cepat dan terpercaya.",
		IsVisible:    true,
	}
	err = repo.Create(r2)
	if err != nil {
		t.Fatalf("failed to create review 2: %v", err)
	}

	// 2. HasOrderReviewed
	hasReviewed, err := repo.HasOrderReviewed("INV-20261013-0001")
	if err != nil || !hasReviewed {
		t.Errorf("expected INV-20261013-0001 to have reviewed: %v", err)
	}
	hasReviewed, _ = repo.HasOrderReviewed("INV-NONEXISTENT")
	if hasReviewed {
		t.Errorf("expected INV-NONEXISTENT not to have reviewed")
	}

	// 3. Stats by product slug
	stats, err := repo.GetStatsByProductSlug("canva_pro")
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.TotalReviews != 2 {
		t.Errorf("expected 2 reviews, got %d", stats.TotalReviews)
	}
	// avg of 5 and 4 = 4.5
	if stats.AverageRating != 4.5 {
		t.Errorf("expected 4.5 avg rating, got %f", stats.AverageRating)
	}
	if stats.Distribution[5] != 1 || stats.Distribution[4] != 1 {
		t.Errorf("unexpected distribution: %+v", stats.Distribution)
	}

	// 4. Update
	r1.Rating = 4
	r1.ReviewText = "Revisi review"
	err = repo.Update(r1)
	if err != nil {
		t.Fatalf("failed to update review: %v", err)
	}

	updated, err := repo.GetByID(r1.ID)
	if err != nil || updated.Rating != 4 || updated.ReviewText != "Revisi review" {
		t.Errorf("update failed: %+v", updated)
	}

	// 5. Delete
	err = repo.Delete(r2.ID)
	if err != nil {
		t.Fatalf("failed to delete review: %v", err)
	}

	list, err := repo.GetByProductSlug("canva_pro", true, 10, 0)
	if err != nil || len(list) != 1 {
		t.Errorf("expected 1 remaining review, got %d", len(list))
	}
}
