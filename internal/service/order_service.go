package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"storeku/internal/models"
	"storeku/internal/repository"
)

type OrderService struct {
	orderRepo      *repository.OrderRepository
	pricing        *PricingService
	dcClient       *DigitalCoreClient
	pgClient       *PaymentGClient
	resendService  *ResendService
	voucherService *VoucherService
	locks          sync.Map // invoice string -> *sync.Mutex
}

func NewOrderService(
	orderRepo *repository.OrderRepository,
	pricing *PricingService,
	dcClient *DigitalCoreClient,
	pgClient *PaymentGClient,
	resendService *ResendService,
) *OrderService {
	return &OrderService{
		orderRepo:     orderRepo,
		pricing:       pricing,
		dcClient:      dcClient,
		pgClient:      pgClient,
		resendService: resendService,
	}
}

func (s *OrderService) SetVoucherService(vs *VoucherService) {
	s.voucherService = vs
}

func (s *OrderService) getInvoiceLock(invoice string) *sync.Mutex {
	l, _ := s.locks.LoadOrStore(invoice, &sync.Mutex{})
	return l.(*sync.Mutex)
}

func generateRandomString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return strings.ToUpper(hex.EncodeToString(bytes)[:n])
}

// CreateCheckout initiates a checkout and generates PaymentG QRIS
func (s *OrderService) CreateCheckout(req *models.CheckoutRequest) (*models.CheckoutResponse, error) {
	if req.ProductSlug == "" {
		return nil, fmt.Errorf("produk harus dipilih")
	}
	req.CustomerEmail = strings.TrimSpace(req.CustomerEmail)
	if req.CustomerEmail == "" || !strings.Contains(req.CustomerEmail, "@") || !strings.Contains(req.CustomerEmail, ".") {
		return nil, fmt.Errorf("alamat email pembeli wajib diisi dengan benar untuk menerima kode klaim")
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	if req.Quantity > 100 {
		return nil, fmt.Errorf("maksimal pembelian 100 item per pesanan")
	}

	// 1. Fetch live product catalog to ensure valid stock and real-time pricing
	products, err := s.dcClient.FetchProducts()
	if err != nil {
		return nil, fmt.Errorf("gagal memuat katalog produk: %w", err)
	}

	var targetProduct *models.Product
	for _, p := range products {
		if p.Slug == req.ProductSlug {
			targetProduct = p
			break
		}
	}

	if targetProduct == nil {
		return nil, fmt.Errorf("produk '%s' tidak ditemukan atau sedang tidak aktif", req.ProductSlug)
	}

	if targetProduct.IsForceOutOfStock {
		return nil, fmt.Errorf("stok produk '%s' sedang habis (dinonaktifkan sementara oleh toko)", targetProduct.Name)
	}

	if targetProduct.Stock > 0 && targetProduct.Stock < req.Quantity {
		return nil, fmt.Errorf("stok tidak mencukupi (sisa: %d)", targetProduct.Stock)
	}

	// 2. Determine price (custom price override takes precedence if configured)
	unitPriceUSD := targetProduct.PriceUSD
	for _, tier := range targetProduct.Tiers {
		if req.Quantity >= tier.MinQty && (tier.MaxQty == nil || req.Quantity <= *tier.MaxQty) {
			unitPriceUSD = tier.PriceUSD
			break
		}
	}

	var unitPriceIDR int64
	if targetProduct.IsCustomized && targetProduct.PriceIDR > 0 {
		unitPriceIDR = targetProduct.PriceIDR
	} else {
		unitPriceIDR = s.pricing.ConvertUSDToIDR(unitPriceUSD)
	}

	// Golden rule safety check: unitPriceIDR must never be less than minSafePrice (modal + margin)
	minSafePrice := s.pricing.ConvertUSDToIDR(unitPriceUSD)
	if unitPriceIDR < minSafePrice {
		unitPriceIDR = minSafePrice
	}

	subtotalIDR := unitPriceIDR * int64(req.Quantity)

	// 2b. Apply voucher discount if supplied
	var appliedVoucherCode string
	var discountIDR int64
	var voucherClaimed bool

	if strings.TrimSpace(req.VoucherCode) != "" && s.voucherService != nil {
		cleanCode := strings.ToUpper(strings.TrimSpace(req.VoucherCode))
		v, disc, errVal := s.voucherService.ValidateForCheckout(cleanCode, subtotalIDR)
		if errVal != nil {
			return nil, fmt.Errorf("voucher tidak dapat digunakan: %w", errVal)
		}

		// Atomically claim 1 quota
		_, errConsume := s.voucherService.ConsumeVoucherAtomic(cleanCode)
		if errConsume != nil {
			return nil, fmt.Errorf("gagal mengklaim kuota voucher: %w", errConsume)
		}
		voucherClaimed = true
		appliedVoucherCode = v.Code
		discountIDR = disc

		// Apply discount with min Rp 1,000 threshold for ShopeePay QRIS
		subtotalIDR = subtotalIDR - discountIDR
		if subtotalIDR < 1000 {
			subtotalIDR = 1000
		}
	}

	// 3. Generate unique Invoice Number and Claim Code
	datePrefix := time.Now().Format("20060102")
	invoiceNumber := fmt.Sprintf("INV-%s-%s", datePrefix, generateRandomString(6))
	claimCode := fmt.Sprintf("CLM-%s", generateRandomString(6))

	// 4. Request QRIS tagihan from PaymentG
	metadata := fmt.Sprintf("Digimest: %s (Qty: %d)", targetProduct.Name, req.Quantity)
	pgOrder, err := s.pgClient.CreateQRISOrder(invoiceNumber, subtotalIDR, metadata)
	if err != nil {
		// Revert voucher usage only if network gateway failure prevented QRIS generation entirely
		if voucherClaimed && s.voucherService != nil {
			_ = s.voucherService.RevertVoucherUsage(appliedVoucherCode)
		}
		return nil, fmt.Errorf("gagal membuat tagihan QRIS PaymentG: %w", err)
	}

	var expiresAtTime *time.Time
	if pgOrder.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, pgOrder.ExpiresAt); err == nil {
			tUTC := t.UTC()
			expiresAtTime = &tUTC
		}
	}
	if expiresAtTime == nil {
		t := time.Now().UTC().Add(15 * time.Minute)
		expiresAtTime = &t
	}

	// 5. Store order in DB
	order := &models.Order{
		InvoiceNumber:     invoiceNumber,
		ClaimCode:         claimCode,
		ProductSlug:       targetProduct.Slug,
		ProductName:       targetProduct.Name,
		ProductType:       targetProduct.Type,
		Quantity:          req.Quantity,
		BasePriceUSD:      unitPriceUSD,
		SellPriceIDR:      unitPriceIDR,
		OriginalAmount:    pgOrder.OriginalAmount,
		UniqueCode:        pgOrder.UniqueCode,
		TotalAmount:       pgOrder.TotalAmount,
		VoucherCode:       appliedVoucherCode,
		DiscountIDR:       discountIDR,
		GatewayOrderID:    pgOrder.OrderID,
		QRURL:             pgOrder.QRURL,
		PaymentStatus:     "PENDING",
		FulfillmentStatus: "PENDING",
		WarrantyInfo:      targetProduct.Warranty,
		CustomerContact:   req.CustomerContact,
		CustomerEmail:     req.CustomerEmail,
		CostAtCheckoutUSD: unitPriceUSD,
		ExpiresAt:         expiresAtTime,
	}

	if err := s.orderRepo.CreateOrder(order); err != nil {
		return nil, fmt.Errorf("gagal menyimpan pesanan ke database: %w", err)
	}

	return &models.CheckoutResponse{
		InvoiceNumber:    invoiceNumber,
		ClaimCode:        "", // Jangan bocorkan kode klaim sebelum lunas
		ProductName:      targetProduct.Name,
		Quantity:         req.Quantity,
		OriginalAmount:   pgOrder.OriginalAmount,
		UniqueCode:       pgOrder.UniqueCode,
		TotalAmount:      pgOrder.TotalAmount,
		VoucherCode:      appliedVoucherCode,
		DiscountIDR:      discountIDR,
		GatewayOrderID:   pgOrder.OrderID,
		QRURL:            pgOrder.QRURL,
		ExpiresAt:        expiresAtTime.Format(time.RFC3339),
		ExpiresInSeconds: pgOrder.ExpiresInSeconds,
	}, nil
}

// GetOrderStatus retrieves status for realtime polling
func (s *OrderService) GetOrderStatus(invoice string) (*models.OrderStatusResponse, error) {
	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return nil, fmt.Errorf("pesanan tidak ditemukan")
	}

	// Auto-expire order if 15-minute window has passed
	if order.PaymentStatus == "PENDING" && order.ExpiresAt != nil && time.Now().UTC().After(order.ExpiresAt.UTC()) {
		_ = s.orderRepo.UpdatePaymentStatus(order.InvoiceNumber, "EXPIRED", nil)
		order.PaymentStatus = "EXPIRED"
	}

	var items []string
	if order.DeliveredItems != "" && order.DeliveredItems != "[]" {
		_ = json.Unmarshal([]byte(order.DeliveredItems), &items)
	}

	// Hanya tampilkan Kode Klaim jika status transaksi sudah LUNAS (PAID)
	visibleClaimCode := ""
	if order.PaymentStatus == "PAID" {
		visibleClaimCode = order.ClaimCode
	}

	res := &models.OrderStatusResponse{
		InvoiceNumber:     order.InvoiceNumber,
		ClaimCode:         visibleClaimCode,
		ProductName:       order.ProductName,
		ProductType:       order.ProductType,
		Quantity:          order.Quantity,
		TotalAmount:       order.TotalAmount,
		PaymentStatus:     order.PaymentStatus,
		FulfillmentStatus: order.FulfillmentStatus,
		QRURL:             order.QRURL,
		DeliveredItems:    items,
		WarrantyInfo:      order.WarrantyInfo,
		CustomerContact:   order.CustomerContact,
		CustomerEmail:     order.CustomerEmail,
	}

	if order.PaidAt != nil {
		res.PaidAt = order.PaidAt.Format(time.RFC3339)
	}
	if order.ExpiresAt != nil {
		res.ExpiresAt = order.ExpiresAt.Format(time.RFC3339)
	}

	if order.FulfillmentStatus == "HOLD_PRICE_SPIKE" {
		res.IsQuotaOut = true
		res.QuotaOutMessage = "Mohon maaf, kuota/stok produk saat ini telah habis terjual. Dana pembayaran Anda aman, untuk proses pengembalian dana (refund) 100% silakan hubungi CS via Telegram."
		res.DeliveredItems = []string{}
	} else if order.FulfillmentStatus == "HOLD_INSUFFICIENT_BOT_BALANCE" || order.FulfillmentStatus == "FAILED" {
		res.IsQuotaOut = true
		res.QuotaOutMessage = "Mohon maaf, terjadi kendala antrean pemenuhan stok otomatis dari penyedia server. Dana pembayaran Anda aman 100%. Silakan hubungi Customer Service via Telegram untuk penanganan kilat atau refund."
		res.DeliveredItems = []string{}
	}

	return res, nil
}

// CancelOrder cancels an unpaid pending order
func (s *OrderService) CancelOrder(invoice string) error {
	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan tidak ditemukan")
	}

	if order.PaymentStatus == "PAID" {
		return fmt.Errorf("pesanan sudah dibayar, tidak dapat dibatalkan")
	}

	if order.PaymentStatus == "CANCELLED" {
		return nil
	}

	return s.orderRepo.UpdatePaymentStatus(order.InvoiceNumber, "CANCELLED", nil)
}

// TriggerInstantCheck triggers immediate mutation check on PaymentG and updates if paid
func (s *OrderService) TriggerInstantCheck(invoice string) error {
	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan tidak ditemukan")
	}

	if order.PaymentStatus == "PAID" {
		return nil
	}

	if order.GatewayOrderID == "" {
		return fmt.Errorf("order belum memiliki gateway_order_id")
	}

	// 1. Call PaymentG check mutation
	_ = s.pgClient.TriggerInstantMutationCheck(order.GatewayOrderID)

	// 2. Query PaymentG order status
	pgOrder, err := s.pgClient.CheckOrderStatus(order.GatewayOrderID)
	if err != nil {
		return err
	}

	if pgOrder.Status == "PAID" {
		return s.ProcessPaymentSuccess(order.InvoiceNumber, pgOrder.ShopeeTxID)
	}

	return nil
}

// ProcessPaymentSuccess handles payment completion and initiates instant fulfillment
func (s *OrderService) ProcessPaymentSuccess(invoice string, txID string) error {
	lock := s.getInvoiceLock(invoice)
	lock.Lock()
	defer lock.Unlock()

	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan '%s' tidak ditemukan", invoice)
	}

	// Idempotency check: if already fulfilled or refunded, don't execute buy upstream again
	if order.FulfillmentStatus == "SUCCESS" || order.FulfillmentStatus == "REFUNDED" {
		log.Printf("[IDEMPOTENT] Order %s fulfillment status is %s. Skipping upstream buy.", invoice, order.FulfillmentStatus)
		return nil
	}

	now := time.Now().UTC()
	if order.PaymentStatus != "PAID" {
		if err := s.orderRepo.UpdatePaymentStatus(invoice, "PAID", &now); err != nil {
			log.Printf("Error updating payment status for %s: %v", invoice, err)
		}

		// Trigger automatic claim code email via Resend in background goroutine
		if s.resendService != nil && order.CustomerEmail != "" {
			go func(toEmail, inv, claim string, total int64, prod string, qty int) {
				if err := s.resendService.SendClaimEmail(toEmail, inv, claim, total, prod, qty); err != nil {
					log.Printf("[RESEND ERROR] Gagal mengirim email ke %s: %v", toEmail, err)
				}
			}(order.CustomerEmail, order.InvoiceNumber, order.ClaimCode, order.TotalAmount, order.ProductName, order.Quantity)
		}
	}

	// PRE-FULFILLMENT CHECK: Check live wholesale cost from DigitalCore before buying
	liveCostUSD, err := s.dcClient.GetProductLiveCost(order.ProductSlug, order.Quantity)
	if err == nil && liveCostUSD > 0 {
		rate := s.pricing.GetUSDRate()
		minMarginPct := s.pricing.GetMarginPercent()
		liveTotalCostIDR := int64(math.Ceil(liveCostUSD * rate * float64(order.Quantity)))

		// 1. Minimum Revenue requirement: ensure default profit margin (e.g. 25%)
		minSafeRevenueIDR := int64(math.Ceil(float64(liveTotalCostIDR) * (1.0 + (minMarginPct / 100.0))))

		// If new wholesale cost exceeds what buyer paid OR dips below safe margin: HALT! Protect bot balance & profit!
		if liveTotalCostIDR > order.TotalAmount || order.TotalAmount < minSafeRevenueIDR {
			errMsg := fmt.Sprintf("Lonjakan harga API upstream: modal naik menjadi Rp %d (keuntungan aman minimal Rp %d, uang bayar Rp %d). Pembelian otomatis ditahan demi perlindungan saldo bot.", liveTotalCostIDR, minSafeRevenueIDR, order.TotalAmount)
			log.Printf("[AUTO-HOLD PRICE SPIKE] %s: %s", invoice, errMsg)
			_ = s.orderRepo.UpdateFulfillment(invoice, "HOLD_PRICE_SPIKE", errMsg, "", "", "[]")
			return nil // SALDO BOT & KEUNTUNGAN AMAN!
		}

		// 2. Pre-check Telegram bot balance before buying
		reqUSD := liveCostUSD * float64(order.Quantity)
		if me, err := s.dcClient.GetUserAccount(); err == nil && me != nil {
			if me.Balance < reqUSD {
				errMsg := fmt.Sprintf("Saldo bot DigitalCore ($%.2f) tidak mencukupi untuk pesanan ini ($%.2f). Pembelian otomatis ditahan untuk top-up saldo bot.", me.Balance, reqUSD)
				log.Printf("[HOLD INSUFFICIENT BOT BALANCE] %s: %s", invoice, errMsg)
				_ = s.orderRepo.UpdateFulfillment(invoice, "HOLD_INSUFFICIENT_BOT_BALANCE", errMsg, "", "", "[]")
				return nil
			}
		}
	}

	// EXECUTE INSTANT AUTO-FULFILLMENT TO DIGITALCORE
	log.Printf("[FULFILLMENT] Initiating auto-buy for %s (Product: %s, Qty: %d)...", invoice, order.ProductSlug, order.Quantity)
	buyResp, err := s.dcClient.BuyProduct(order.ProductSlug, order.Quantity)
	if err != nil {
		log.Printf("[FULFILLMENT ERROR] Order %s failed to buy from DigitalCore: %v", invoice, err)
		_ = s.orderRepo.UpdateFulfillment(invoice, "FAILED", err.Error(), "", "", "[]")
		return nil
	}

	itemsJSON, _ := json.Marshal(buyResp.Items)
	if err := s.orderRepo.UpdateFulfillmentWithCost(invoice, "SUCCESS", "", buyResp.OrderID, buyResp.ViewURL, string(itemsJSON), liveCostUSD); err != nil {
		log.Printf("Error updating fulfillment record for %s: %v", invoice, err)
	}

	log.Printf("[FULFILLMENT SUCCESS] Order %s fulfilled with %d item(s). DC Order ID: %s", invoice, len(buyResp.Items), buyResp.OrderID)
	return nil
}

// RetryFulfillment allows admin to manually retry purchase if previously failed (e.g. after topping up bot balance)
func (s *OrderService) RetryFulfillment(invoice string) error {
	return s.ForceFulfillOrder(invoice)
}

// ForceFulfillOrder allows admin to consciously fulfill an order even if held
func (s *OrderService) ForceFulfillOrder(invoice string) error {
	lock := s.getInvoiceLock(invoice)
	lock.Lock()
	defer lock.Unlock()

	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan tidak ditemukan")
	}

	if order.PaymentStatus != "PAID" {
		return fmt.Errorf("pesanan belum berstatus PAID")
	}

	if order.FulfillmentStatus == "SUCCESS" || order.FulfillmentStatus == "REFUNDED" {
		return fmt.Errorf("pesanan sudah berstatus %s, tidak dapat dipenuhi", order.FulfillmentStatus)
	}

	log.Printf("[FORCE FULFILL] Admin forced buy for %s (Product: %s, Qty: %d)...", invoice, order.ProductSlug, order.Quantity)
	buyResp, err := s.dcClient.BuyProduct(order.ProductSlug, order.Quantity)
	if err != nil {
		_ = s.orderRepo.UpdateFulfillment(invoice, "FAILED", err.Error(), "", "", "[]")
		return fmt.Errorf("gagal memenuhi stok dari DigitalCore: %w", err)
	}

	itemsJSON, _ := json.Marshal(buyResp.Items)
	if err := s.orderRepo.UpdateFulfillmentWithCost(invoice, "SUCCESS", "", buyResp.OrderID, buyResp.ViewURL, string(itemsJSON), 0); err != nil {
		return err
	}

	return nil
}

// ManualMarkPaidAndFulfill allows admin to confirm manual transfer without unique code and instantly fulfill
func (s *OrderService) ManualMarkPaidAndFulfill(invoice string) error {
	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan '%s' tidak ditemukan", invoice)
	}
	if order.FulfillmentStatus == "SUCCESS" || order.FulfillmentStatus == "REFUNDED" {
		return fmt.Errorf("pesanan sudah berstatus %s, tidak dapat dipenuhi manual", order.FulfillmentStatus)
	}

	now := time.Now().UTC()
	if err := s.orderRepo.UpdatePaymentStatus(invoice, "PAID", &now); err != nil {
		return fmt.Errorf("gagal mengupdate status pesanan menjadi PAID: %w", err)
	}

	return s.ProcessPaymentSuccess(invoice, "MANUAL_ADMIN_CONFIRMATION")
}

// MarkRefundedOrder marks an order fulfillment status as REFUNDED
func (s *OrderService) MarkRefundedOrder(invoice string) error {
	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan tidak ditemukan")
	}

	if order.FulfillmentStatus == "SUCCESS" {
		return fmt.Errorf("pesanan yang sudah berhasil dikirim tidak dapat ditandai refund")
	}

	_ = s.orderRepo.UpdatePaymentStatus(invoice, "REFUNDED", order.PaidAt)
	return s.orderRepo.UpdateFulfillment(invoice, "REFUNDED", "Refund dana telah diselesaikan oleh admin", "", "", "[]")
}

// ClaimProduct finds license keys by claim code or invoice number
func (s *OrderService) ClaimProduct(query string) (*models.ClaimResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("kode klaim atau nomor invoice wajib diisi")
	}

	var order *models.Order
	var err error

	// If starts with CLM or is short, try claim code first
	if strings.HasPrefix(strings.ToUpper(query), "CLM-") || len(query) < 14 {
		order, err = s.orderRepo.GetOrderByClaimCode(strings.ToUpper(query))
	}
	if order == nil {
		order, err = s.orderRepo.GetOrderByInvoice(strings.ToUpper(query))
	}
	if order == nil {
		// Fallback check claim code directly
		order, err = s.orderRepo.GetOrderByClaimCode(query)
	}

	if err != nil || order == nil {
		return nil, fmt.Errorf("pesanan dengan kode '%s' tidak ditemukan", query)
	}

	var items []string
	if order.DeliveredItems != "" && order.DeliveredItems != "[]" {
		_ = json.Unmarshal([]byte(order.DeliveredItems), &items)
	}

	var isQuotaOut bool
	var quotaOutMsg string
	if order.FulfillmentStatus == "HOLD_PRICE_SPIKE" {
		isQuotaOut = true
		quotaOutMsg = "Mohon maaf, kuota/stok produk saat ini telah habis terjual. Dana pembayaran Anda aman, untuk proses pengembalian dana (refund) 100% silakan hubungi CS via Telegram."
		items = []string{}
	} else if order.FulfillmentStatus == "HOLD_INSUFFICIENT_BOT_BALANCE" || order.FulfillmentStatus == "FAILED" {
		isQuotaOut = true
		quotaOutMsg = "Mohon maaf, terjadi kendala antrean pemenuhan stok otomatis dari penyedia server. Dana pembayaran Anda aman 100%. Silakan hubungi CS kami via Telegram untuk penanganan prioritas atau refund."
		items = []string{}
	}

	// Jangan bocorkan kode klaim jika order belum lunas (misal dicari via invoice)
	visibleClaimCode := ""
	if order.PaymentStatus == "PAID" {
		visibleClaimCode = order.ClaimCode
	}

	return &models.ClaimResponse{
		Success:            true,
		InvoiceNumber:      order.InvoiceNumber,
		ClaimCode:          visibleClaimCode,
		ProductName:        order.ProductName,
		ProductType:        order.ProductType,
		Quantity:           order.Quantity,
		PaymentStatus:      order.PaymentStatus,
		FulfillmentStatus:  order.FulfillmentStatus,
		DeliveredItems:     items,
		WarrantyInfo:       order.WarrantyInfo,
		DigitalCoreViewURL: "", // Jangan bocorkan link upstream ke pembeli, cukup di admin panel
		CreatedAt:          order.CreatedAt.Format("02 Jan 2006, 15:04"),
		IsQuotaOut:         isQuotaOut,
		QuotaOutMessage:    quotaOutMsg,
	}, nil
}
