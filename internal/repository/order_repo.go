package repository

import (
	"database/sql"
	"fmt"
	"time"

	"storeku/internal/database"
	"storeku/internal/models"
)

type OrderRepository struct {
	db *database.DB
}

func NewOrderRepository(db *database.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) CreateOrder(o *models.Order) error {
	query := `
		INSERT INTO orders (
			invoice_number, claim_code, product_slug, product_name, product_type,
			quantity, base_price_usd, sell_price_idr, original_amount, unique_code,
			total_amount, gateway_order_id, qr_url, payment_status, fulfillment_status,
			warranty_info, customer_contact, customer_email, expires_at, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`
	res, err := r.db.Exec(
		query,
		o.InvoiceNumber, o.ClaimCode, o.ProductSlug, o.ProductName, o.ProductType,
		o.Quantity, o.BasePriceUSD, o.SellPriceIDR, o.OriginalAmount, o.UniqueCode,
		o.TotalAmount, o.GatewayOrderID, o.QRURL, o.PaymentStatus, o.FulfillmentStatus,
		o.WarrantyInfo, o.CustomerContact, o.CustomerEmail, o.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		o.ID = id
	}
	return nil
}

func (r *OrderRepository) GetOrderByInvoice(invoice string) (*models.Order, error) {
	query := `
		SELECT
			id, invoice_number, claim_code, product_slug, product_name, product_type,
			quantity, base_price_usd, sell_price_idr, original_amount, unique_code,
			total_amount, gateway_order_id, qr_url, payment_status, fulfillment_status,
			fulfillment_error, digitalcore_order_id, digitalcore_view_url, delivered_items,
			warranty_info, customer_contact, customer_email, paid_at, expires_at, created_at, updated_at
		FROM orders WHERE invoice_number = ?
	`
	row := r.db.QueryRow(query, invoice)
	return scanOrder(row)
}

func (r *OrderRepository) GetOrderByClaimCode(claimCode string) (*models.Order, error) {
	query := `
		SELECT
			id, invoice_number, claim_code, product_slug, product_name, product_type,
			quantity, base_price_usd, sell_price_idr, original_amount, unique_code,
			total_amount, gateway_order_id, qr_url, payment_status, fulfillment_status,
			fulfillment_error, digitalcore_order_id, digitalcore_view_url, delivered_items,
			warranty_info, customer_contact, customer_email, paid_at, expires_at, created_at, updated_at
		FROM orders WHERE claim_code = ?
	`
	row := r.db.QueryRow(query, claimCode)
	return scanOrder(row)
}

func (r *OrderRepository) GetOrderByGatewayOrderID(gwID string) (*models.Order, error) {
	query := `
		SELECT
			id, invoice_number, claim_code, product_slug, product_name, product_type,
			quantity, base_price_usd, sell_price_idr, original_amount, unique_code,
			total_amount, gateway_order_id, qr_url, payment_status, fulfillment_status,
			fulfillment_error, digitalcore_order_id, digitalcore_view_url, delivered_items,
			warranty_info, customer_contact, customer_email, paid_at, expires_at, created_at, updated_at
		FROM orders WHERE gateway_order_id = ?
	`
	row := r.db.QueryRow(query, gwID)
	return scanOrder(row)
}

func (r *OrderRepository) UpdatePaymentStatus(invoice, status string, paidAt *time.Time) error {
	query := `
		UPDATE orders
		SET payment_status = ?, paid_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE invoice_number = ?
	`
	_, err := r.db.Exec(query, status, paidAt, invoice)
	return err
}

func (r *OrderRepository) UpdateGatewayDetails(invoice, gwID, qrURL string, uniqueCode int, totalAmount int64, expiresAt *time.Time) error {
	query := `
		UPDATE orders
		SET gateway_order_id = ?, qr_url = ?, unique_code = ?, total_amount = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE invoice_number = ?
	`
	_, err := r.db.Exec(query, gwID, qrURL, uniqueCode, totalAmount, expiresAt, invoice)
	return err
}

func (r *OrderRepository) UpdateFulfillment(invoice, status, errStr, dcOrderID, dcViewURL, deliveredJSON string) error {
	query := `
		UPDATE orders
		SET fulfillment_status = ?, fulfillment_error = ?, digitalcore_order_id = ?,
		    digitalcore_view_url = ?, delivered_items = ?, updated_at = CURRENT_TIMESTAMP
		WHERE invoice_number = ?
	`
	_, err := r.db.Exec(query, status, errStr, dcOrderID, dcViewURL, deliveredJSON, invoice)
	return err
}

func (r *OrderRepository) ListOrders(limit, offset int, statusFilter string) ([]*models.Order, int64, error) {
	countQuery := "SELECT COUNT(*) FROM orders"
	dataQuery := `
		SELECT
			id, invoice_number, claim_code, product_slug, product_name, product_type,
			quantity, base_price_usd, sell_price_idr, original_amount, unique_code,
			total_amount, gateway_order_id, qr_url, payment_status, fulfillment_status,
			fulfillment_error, digitalcore_order_id, digitalcore_view_url, delivered_items,
			warranty_info, customer_contact, customer_email, paid_at, expires_at, created_at, updated_at
		FROM orders
	`

	var args []interface{}
	var countArgs []interface{}
	if statusFilter != "" && statusFilter != "ALL" {
		countQuery += " WHERE payment_status = ?"
		dataQuery += " WHERE payment_status = ?"
		args = append(args, statusFilter)
		countArgs = append(countArgs, statusFilter)
	}

	var total int64
	if err := r.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.Query(dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		o, err := scanOrderRows(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}

	return orders, total, nil
}

func (r *OrderRepository) GetAdminStats() (*models.AdminStats, error) {
	stats := &models.AdminStats{}

	// Total and status counts
	row := r.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN payment_status = 'PAID' THEN total_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN payment_status = 'PAID' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN payment_status = 'PENDING' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN fulfillment_status = 'FAILED' THEN 1 ELSE 0 END), 0)
		FROM orders
	`)
	if err := row.Scan(&stats.TotalOrders, &stats.TotalRevenueIDR, &stats.PaidOrders, &stats.PendingOrders, &stats.FailedOrders); err != nil {
		return nil, err
	}

	// Today stats
	todayRow := r.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN payment_status = 'PAID' THEN total_amount ELSE 0 END), 0)
		FROM orders
		WHERE DATE(created_at) = DATE('now')
	`)
	_ = todayRow.Scan(&stats.TodayOrders, &stats.TodayRevenueIDR)

	return stats, nil
}

func scanOrder(row *sql.Row) (*models.Order, error) {
	o := &models.Order{}
	err := row.Scan(
		&o.ID, &o.InvoiceNumber, &o.ClaimCode, &o.ProductSlug, &o.ProductName, &o.ProductType,
		&o.Quantity, &o.BasePriceUSD, &o.SellPriceIDR, &o.OriginalAmount, &o.UniqueCode,
		&o.TotalAmount, &o.GatewayOrderID, &o.QRURL, &o.PaymentStatus, &o.FulfillmentStatus,
		&o.FulfillmentError, &o.DigitalCoreOrderID, &o.DigitalCoreViewURL, &o.DeliveredItems,
		&o.WarrantyInfo, &o.CustomerContact, &o.CustomerEmail, &o.PaidAt, &o.ExpiresAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func scanOrderRows(rows *sql.Rows) (*models.Order, error) {
	o := &models.Order{}
	err := rows.Scan(
		&o.ID, &o.InvoiceNumber, &o.ClaimCode, &o.ProductSlug, &o.ProductName, &o.ProductType,
		&o.Quantity, &o.BasePriceUSD, &o.SellPriceIDR, &o.OriginalAmount, &o.UniqueCode,
		&o.TotalAmount, &o.GatewayOrderID, &o.QRURL, &o.PaymentStatus, &o.FulfillmentStatus,
		&o.FulfillmentError, &o.DigitalCoreOrderID, &o.DigitalCoreViewURL, &o.DeliveredItems,
		&o.WarrantyInfo, &o.CustomerContact, &o.CustomerEmail, &o.PaidAt, &o.ExpiresAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}
