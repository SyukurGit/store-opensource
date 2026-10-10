package database

import (
	"database/sql"
	"fmt"
)

func RunMigrations(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			invoice_number TEXT UNIQUE NOT NULL,
			claim_code TEXT UNIQUE NOT NULL,
			product_slug TEXT NOT NULL,
			product_name TEXT NOT NULL,
			product_type TEXT DEFAULT '',
			quantity INTEGER NOT NULL DEFAULT 1,
			base_price_usd REAL NOT NULL DEFAULT 0,
			sell_price_idr INTEGER NOT NULL DEFAULT 0,
			original_amount INTEGER NOT NULL DEFAULT 0,
			unique_code INTEGER NOT NULL DEFAULT 0,
			total_amount INTEGER NOT NULL DEFAULT 0,
			gateway_order_id TEXT DEFAULT '',
			qr_url TEXT DEFAULT '',
			payment_status TEXT NOT NULL DEFAULT 'PENDING',
			fulfillment_status TEXT NOT NULL DEFAULT 'PENDING',
			fulfillment_error TEXT DEFAULT '',
			digitalcore_order_id TEXT DEFAULT '',
			digitalcore_view_url TEXT DEFAULT '',
			delivered_items TEXT DEFAULT '[]',
			warranty_info TEXT DEFAULT '',
			customer_contact TEXT DEFAULT '',
			customer_email TEXT DEFAULT '',
			paid_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS product_overrides (
			product_slug TEXT PRIMARY KEY,
			custom_name TEXT DEFAULT '',
			custom_price_idr INTEGER DEFAULT 0,
			promo_price_idr INTEGER DEFAULT 0,
			custom_description TEXT DEFAULT '',
			custom_terms TEXT DEFAULT '',
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_orders_invoice ON orders(invoice_number);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_claim ON orders(claim_code);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_gateway ON orders(gateway_order_id);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_payment_status ON orders(payment_status);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_product_overrides_active ON product_overrides(is_active);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migration query failed: %w (query: %s)", err, q)
		}
	}

	// Safe column addition for existing sqlite database
	_, _ = db.Exec("ALTER TABLE orders ADD COLUMN customer_email TEXT DEFAULT '';")
	_, _ = db.Exec("ALTER TABLE orders ADD COLUMN cost_at_checkout_usd REAL DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE orders ADD COLUMN cost_at_fulfillment_usd REAL DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE product_overrides ADD COLUMN promo_price_idr INTEGER DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE product_overrides ADD COLUMN baseline_modal_idr INTEGER DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE product_overrides ADD COLUMN is_auto_suspended INTEGER DEFAULT 0;")

	return nil
}
