package database

import (
	"database/sql"
	"fmt"
)

func RunMigrations(db *sql.DB) error {
	tableQueries := []string{
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
			cost_at_checkout_usd REAL DEFAULT 0,
			cost_at_fulfillment_usd REAL DEFAULT 0,
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
			custom_type TEXT DEFAULT '',
			custom_region TEXT DEFAULT '',
			custom_warranty TEXT DEFAULT '',
			custom_stock INTEGER NOT NULL DEFAULT -1,
			custom_price_idr INTEGER DEFAULT 0,
			promo_price_idr INTEGER DEFAULT 0,
			custom_description TEXT DEFAULT '',
			custom_terms TEXT DEFAULT '',
			is_active INTEGER NOT NULL DEFAULT 1,
			is_force_out_of_stock INTEGER NOT NULL DEFAULT 0,
			baseline_modal_idr INTEGER DEFAULT 0,
			is_auto_suspended INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS cached_products (
			slug TEXT PRIMARY KEY,
			raw_id TEXT DEFAULT '',
			name TEXT NOT NULL,
			type TEXT DEFAULT '',
			logo TEXT DEFAULT '',
			region TEXT DEFAULT '',
			stock INTEGER NOT NULL DEFAULT 0,
			stock_capped INTEGER NOT NULL DEFAULT 0,
			warranty TEXT DEFAULT '',
			description TEXT DEFAULT '',
			price_usd REAL NOT NULL DEFAULT 0,
			tiers_json TEXT DEFAULT '[]',
			last_fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS admin_login_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			user_agent TEXT NOT NULL,
			location TEXT DEFAULT '',
			device_info TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'SUCCESS',
			failure_reason TEXT DEFAULT '',
			login_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS vouchers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT UNIQUE NOT NULL,
			discount_idr INTEGER NOT NULL DEFAULT 0,
			min_spend_idr INTEGER NOT NULL DEFAULT 0,
			max_usage INTEGER NOT NULL DEFAULT 1,
			used_count INTEGER NOT NULL DEFAULT 0,
			expires_at DATETIME,
			is_active INTEGER NOT NULL DEFAULT 1,
			signature TEXT NOT NULL DEFAULT '',
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS local_products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			type TEXT DEFAULT 'Digital License',
			logo TEXT DEFAULT '',
			region TEXT DEFAULT 'Global',
			warranty TEXT DEFAULT 'Full Garansi',
			description TEXT DEFAULT '',
			terms TEXT DEFAULT 'Membeli berarti setuju dengan syarat & ketentuan',
			price_idr INTEGER NOT NULL DEFAULT 0,
			reference_price_idr INTEGER NOT NULL DEFAULT 0,
			promo_price_idr INTEGER NOT NULL DEFAULT 0,
			has_promo INTEGER NOT NULL DEFAULT 0,
			discount_percent INTEGER NOT NULL DEFAULT 0,
			stock INTEGER NOT NULL DEFAULT 0,
			stock_keys TEXT NOT NULL DEFAULT '[]',
			is_active INTEGER NOT NULL DEFAULT 1,
			is_force_out_of_stock INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range tableQueries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migration table query failed: %w (query: %s)", err, q)
		}
	}

	// Safe column additions for existing sqlite database (run before creating indexes on new columns)
	alterQueries := []string{
		"ALTER TABLE orders ADD COLUMN customer_email TEXT DEFAULT '';",
		"ALTER TABLE orders ADD COLUMN cost_at_checkout_usd REAL DEFAULT 0;",
		"ALTER TABLE orders ADD COLUMN cost_at_fulfillment_usd REAL DEFAULT 0;",
		"ALTER TABLE orders ADD COLUMN voucher_code TEXT DEFAULT '';",
		"ALTER TABLE orders ADD COLUMN discount_idr INTEGER DEFAULT 0;",
		"ALTER TABLE product_overrides ADD COLUMN promo_price_idr INTEGER DEFAULT 0;",
		"ALTER TABLE product_overrides ADD COLUMN baseline_modal_idr INTEGER DEFAULT 0;",
		"ALTER TABLE product_overrides ADD COLUMN is_auto_suspended INTEGER DEFAULT 0;",
		"ALTER TABLE product_overrides ADD COLUMN is_force_out_of_stock INTEGER DEFAULT 0;",
		"ALTER TABLE product_overrides ADD COLUMN custom_type TEXT DEFAULT '';",
		"ALTER TABLE product_overrides ADD COLUMN custom_region TEXT DEFAULT '';",
		"ALTER TABLE product_overrides ADD COLUMN custom_warranty TEXT DEFAULT '';",
		"ALTER TABLE product_overrides ADD COLUMN custom_stock INTEGER DEFAULT -1;",
		"ALTER TABLE admin_login_logs ADD COLUMN status TEXT DEFAULT 'SUCCESS';",
		"ALTER TABLE admin_login_logs ADD COLUMN failure_reason TEXT DEFAULT '';",
	}
	for _, alterQ := range alterQueries {
		_, _ = db.Exec(alterQ)
	}

	// Create indexes AFTER tables and altered columns are guaranteed to exist
	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_invoice ON orders(invoice_number);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_claim ON orders(claim_code);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_gateway ON orders(gateway_order_id);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_payment_status ON orders(payment_status);`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_product_overrides_active ON product_overrides(is_active);`,
		`CREATE INDEX IF NOT EXISTS idx_admin_login_at ON admin_login_logs(login_at);`,
		`CREATE INDEX IF NOT EXISTS idx_admin_login_status ON admin_login_logs(status);`,
		`CREATE INDEX IF NOT EXISTS idx_admin_login_ip ON admin_login_logs(ip_address);`,
		`CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(code);`,
		`CREATE INDEX IF NOT EXISTS idx_vouchers_active ON vouchers(is_active);`,
		`CREATE INDEX IF NOT EXISTS idx_local_products_slug ON local_products(slug);`,
		`CREATE INDEX IF NOT EXISTS idx_local_products_active ON local_products(is_active);`,
	}

	for _, idxQ := range indexQueries {
		if _, err := db.Exec(idxQ); err != nil {
			return fmt.Errorf("migration index query failed: %w (query: %s)", err, idxQ)
		}
	}

	return nil
}

