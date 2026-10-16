package repository

import (
	"path/filepath"
	"testing"

	"storeku/internal/database"
	"storeku/internal/models"
)

func TestMaintenanceRepository(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_maintenance.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	defer db.Close()

	repo := NewMaintenanceRepository(db)

	// 1. Insert sample orders (1 recent, 1 old expired, 1 old paid with keys)
	_, err = db.Exec(`
		INSERT INTO orders (invoice_number, claim_code, product_slug, product_name, quantity, total_amount, payment_status, delivered_items, created_at)
		VALUES 
		('INV-RECENT', 'CLM-RECENT', 'prod-1', 'Product 1', 1, 10000, 'PAID', '["KEY-1"]', datetime('now')),
		('INV-OLD-EXP', 'CLM-OLD-EXP', 'prod-2', 'Product 2', 1, 20000, 'EXPIRED', '[]', datetime('now', '-5 days')),
		('INV-OLD-PAID', 'CLM-OLD-PAID', 'prod-3', 'Product 3', 1, 30000, 'PAID', '["KEY-2", "KEY-3"]', datetime('now', '-10 days'));
	`)
	if err != nil {
		t.Fatalf("failed to insert test orders: %v", err)
	}

	// 2. Insert sample login logs
	_, err = db.Exec(`
		INSERT INTO admin_login_logs (username, ip_address, user_agent, status, login_at)
		VALUES 
		('admin', '127.0.0.1', 'test-agent', 'SUCCESS', datetime('now')),
		('admin', '10.0.0.1', 'hacker-agent', 'FAILED', datetime('now', '-10 days'));
	`)
	if err != nil {
		t.Fatalf("failed to insert test login logs: %v", err)
	}

	// 3. Insert sample vouchers
	_, err = db.Exec(`
		INSERT INTO vouchers (code, discount_idr, min_spend_idr, max_usage, used_count, is_active)
		VALUES 
		('ACTIVE10', 10000, 50000, 100, 2, 1),
		('EXPIRED10', 10000, 50000, 5, 5, 1);
	`)
	if err != nil {
		t.Fatalf("failed to insert test vouchers: %v", err)
	}

	// 4. Test GetStats
	stats, err := repo.GetStats(dbPath)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.TotalOrders != 3 {
		t.Fatalf("expected 3 total orders, got %d", stats.TotalOrders)
	}
	if stats.ExpiredCancelledOrders != 1 {
		t.Fatalf("expected 1 expired order, got %d", stats.ExpiredCancelledOrders)
	}
	if stats.OrdersWithKeysOlder3Days != 1 {
		t.Fatalf("expected 1 order with keys older than 3 days, got %d", stats.OrdersWithKeysOlder3Days)
	}
	if stats.TotalLoginLogs != 2 {
		t.Fatalf("expected 2 login logs, got %d", stats.TotalLoginLogs)
	}
	if stats.FailedLoginLogs != 1 {
		t.Fatalf("expected 1 failed login log, got %d", stats.FailedLoginLogs)
	}
	if stats.ExpiredUsedVouchers != 1 {
		t.Fatalf("expected 1 expired/used voucher, got %d", stats.ExpiredUsedVouchers)
	}

	// 5. Test Clean Delivered Keys older than 3 days
	cleanKeysReq := &models.DatabaseCleanRequest{
		CleanDeliveredKeys: true,
		KeysOlderDays:      3,
	}
	resKeys, err := repo.CleanDatabase(dbPath, cleanKeysReq)
	if err != nil {
		t.Fatalf("failed to clean keys: %v", err)
	}
	if resKeys.KeysCleaned != 1 {
		t.Fatalf("expected 1 key cleaned, got %d", resKeys.KeysCleaned)
	}

	// Verify INV-OLD-PAID delivered_items is now empty
	var keysOld string
	_ = db.QueryRow("SELECT delivered_items FROM orders WHERE invoice_number = 'INV-OLD-PAID'").Scan(&keysOld)
	if keysOld != "[]" {
		t.Fatalf("expected delivered_items to be '[]', got: %s", keysOld)
	}

	// Verify INV-RECENT delivered_items is still intact
	var keysRecent string
	_ = db.QueryRow("SELECT delivered_items FROM orders WHERE invoice_number = 'INV-RECENT'").Scan(&keysRecent)
	if keysRecent != `["KEY-1"]` {
		t.Fatalf("expected recent delivered_items to be intact, got: %s", keysRecent)
	}

	// 6. Test Clean Expired/Cancelled Orders
	cleanOrdersReq := &models.DatabaseCleanRequest{
		CleanOrders: true,
		OrdersMode:  "EXPIRED_CANCELLED",
		RunVacuum:   true,
	}
	resOrders, err := repo.CleanDatabase(dbPath, cleanOrdersReq)
	if err != nil {
		t.Fatalf("failed to clean expired orders: %v", err)
	}
	if resOrders.OrdersDeleted != 1 {
		t.Fatalf("expected 1 order deleted, got %d", resOrders.OrdersDeleted)
	}
	if !resOrders.VacuumExecuted {
		t.Fatalf("expected VACUUM to execute successfully")
	}

	// 7. Test Clean Login Logs (Failed only)
	cleanLogsReq := &models.DatabaseCleanRequest{
		CleanLoginLogs: true,
		LogsMode:       "FAILED_ONLY",
	}
	resLogs, err := repo.CleanDatabase(dbPath, cleanLogsReq)
	if err != nil {
		t.Fatalf("failed to clean logs: %v", err)
	}
	if resLogs.LogsDeleted != 1 {
		t.Fatalf("expected 1 log deleted, got %d", resLogs.LogsDeleted)
	}

	// 8. Test Clean Vouchers (Expired/used only)
	cleanVouchersReq := &models.DatabaseCleanRequest{
		CleanVouchers: true,
		VouchersMode:  "EXPIRED_USED",
	}
	resVouchers, err := repo.CleanDatabase(dbPath, cleanVouchersReq)
	if err != nil {
		t.Fatalf("failed to clean vouchers: %v", err)
	}
	if resVouchers.VouchersDeleted != 1 {
		t.Fatalf("expected 1 voucher deleted, got %d", resVouchers.VouchersDeleted)
	}

	// 9. Test Aliases from frontend (orders_scope, vacuum, size_before, etc.)
	aliasReq := &models.DatabaseCleanRequest{
		CleanOrders: true,
		OrdersScope: "all",
		Vacuum:      true,
	}
	resAlias, err := repo.CleanDatabase(dbPath, aliasReq)
	if err != nil {
		t.Fatalf("failed to clean with aliases: %v", err)
	}
	if resAlias.OrdersDeleted < 1 {
		t.Fatalf("expected orders to be deleted with orders_scope='all', got %d", resAlias.OrdersDeleted)
	}
	if !resAlias.Vacuumed {
		t.Fatalf("expected Vacuumed alias to be true")
	}
	if resAlias.SizeBefore == "" || resAlias.SizeAfter == "" {
		t.Fatalf("expected SizeBefore and SizeAfter to be populated")
	}
}

func TestOrderRepository_Delete(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_order_delete.db")

	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	defer db.Close()

	orderRepo := NewOrderRepository(db)

	// Insert test order
	_, err = db.Exec(`
		INSERT INTO orders (invoice_number, claim_code, product_slug, product_name, quantity, total_amount, payment_status, created_at)
		VALUES ('INV-TEST-DEL', 'CLM-TEST-DEL', 'prod-del', 'Product Del', 1, 15000, 'PENDING', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("failed to insert test order: %v", err)
	}

	// Verify order exists
	ord, err := orderRepo.GetOrderByInvoice("INV-TEST-DEL")
	if err != nil || ord == nil {
		t.Fatalf("expected order to exist: %v", err)
	}

	// Delete order
	if err := orderRepo.Delete("INV-TEST-DEL"); err != nil {
		t.Fatalf("failed to delete order: %v", err)
	}

	// Verify order is gone
	ordAfter, err := orderRepo.GetOrderByInvoice("INV-TEST-DEL")
	if err == nil || ordAfter != nil {
		t.Fatalf("expected order to be deleted, but still found it")
	}

	// Deleting again should return not found error
	if err := orderRepo.Delete("INV-TEST-DEL"); err == nil {
		t.Fatalf("expected error when deleting non-existent order, got nil")
	}
}
