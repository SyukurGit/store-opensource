package service

import (
	"context"
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
	"storeku/internal/payment"
	"storeku/internal/payment/paymentg"
	"storeku/internal/provider"
	"storeku/internal/repository"
)

type OrderService struct {
	orderRepo        *repository.OrderRepository
	pricing          *PricingService
	dcClient         *DigitalCoreClient
	pgClient         *PaymentGClient
	paymentRegistry  *payment.Registry
	resendService    *ResendService
	voucherService   *VoucherService
	localProductRepo *repository.LocalProductRepository
	providerRegistry *provider.Registry
	catalogService   *CatalogService
	locks            sync.Map // invoice string -> *sync.Mutex
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

func (s *OrderService) SetPaymentRegistry(pr *payment.Registry) {
	s.paymentRegistry = pr
}

func (s *OrderService) getPaymentGateway(gwID string) (payment.Gateway, error) {
	if s.paymentRegistry != nil {
		if gwID != "" {
			if gw, ok := s.paymentRegistry.Get(gwID); ok {
				return gw, nil
			}
		}
		if def, err := s.paymentRegistry.Default(); err == nil {
			return def, nil
		}
	}
	if s.pgClient != nil {
		return paymentg.NewAdapterWithClient(s.pgClient.Config(), s.pgClient.Client()), nil
	}
	return nil, fmt.Errorf("tidak ada gateway pembayaran yang terkonfigurasi")
}

func (s *OrderService) SetVoucherService(vs *VoucherService) {
	s.voucherService = vs
}

func (s *OrderService) SetLocalProductRepo(lpr *repository.LocalProductRepository) {
	s.localProductRepo = lpr
}

func (s *OrderService) SetProviderRegistry(pr *provider.Registry) {
	s.providerRegistry = pr
}

func (s *OrderService) SetCatalogService(cs *CatalogService) {
	s.catalogService = cs
}

// GetOrderByInvoice retrieves an order by its invoice number
func (s *OrderService) GetOrderByInvoice(invoice string) (*models.Order, error) {
	return s.orderRepo.GetOrderByInvoice(invoice)
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

// resolveProviderID resolves the provider name from product slug or registered adapters
func (s *OrderService) resolveProviderID(productSlug string) string {
	if strings.HasPrefix(productSlug, "silvex_") {
		return "SILVEX"
	}
	return "DIGITALCORE"
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
	var targetProduct *models.Product
	if s.catalogService != nil {
		targetProduct, _ = s.catalogService.GetPublicProductBySlug(context.Background(), req.ProductSlug, "id")
	}

	if targetProduct == nil && s.localProductRepo != nil {
		if lp, err := s.localProductRepo.GetBySlug(req.ProductSlug); err == nil && lp != nil && lp.IsActive {
			targetProduct = lp.ToProduct()
		}
	}

	if targetProduct == nil && s.providerRegistry != nil {
		provID := s.resolveProviderID(req.ProductSlug)
		if prov, ok := s.providerRegistry.Get(provID); ok {
			if prods, err := prov.FetchCatalog(context.Background()); err == nil {
				for _, p := range prods {
					if p.Slug == req.ProductSlug {
						targetProduct = p
						break
					}
				}
			}
		}
	}

	if targetProduct == nil && s.dcClient != nil {
		if products, err := s.dcClient.FetchProducts(); err == nil {
			for _, p := range products {
				if p.Slug == req.ProductSlug {
					targetProduct = p
					break
				}
			}
		}
	}

	if targetProduct == nil {
		return nil, fmt.Errorf("produk '%s' tidak ditemukan atau sedang tidak aktif", req.ProductSlug)
	}

	if targetProduct.IsForceOutOfStock {
		return nil, fmt.Errorf("stok produk '%s' sedang habis (dinonaktifkan sementara oleh toko)", targetProduct.Name)
	}

	if targetProduct.Source == "LOCAL" && targetProduct.Stock <= 0 {
		return nil, fmt.Errorf("stok produk '%s' saat ini sedang habis", targetProduct.Name)
	}

	if targetProduct.Stock > 0 && targetProduct.Stock < req.Quantity {
		return nil, fmt.Errorf("stok tidak mencukupi (sisa: %d)", targetProduct.Stock)
	}

	// 2. Determine price
	var unitPriceUSD float64
	var unitPriceIDR int64

	if targetProduct.Source == "LOCAL" {
		unitPriceUSD = 0
		unitPriceIDR = targetProduct.PriceIDR
	} else {
		unitPriceUSD = targetProduct.PriceUSD
		for _, tier := range targetProduct.Tiers {
			if req.Quantity >= tier.MinQty && (tier.MaxQty == nil || req.Quantity <= *tier.MaxQty) {
				unitPriceUSD = tier.PriceUSD
				break
			}
		}

		if (targetProduct.IsCustomized || targetProduct.HasPromo) && targetProduct.PriceIDR > 0 {
			unitPriceIDR = targetProduct.PriceIDR
		} else {
			unitPriceIDR = s.pricing.ConvertUSDToIDR(unitPriceUSD)
		}

		// Floor check: unitPriceIDR must never be less than modalIDR
		modalFloorIDR := targetProduct.ModalIDR
		if modalFloorIDR <= 0 && s.pricing != nil {
			modalFloorIDR = int64(math.Round(unitPriceUSD * s.pricing.GetUSDRate()))
		}
		if unitPriceIDR < modalFloorIDR {
			unitPriceIDR = modalFloorIDR
		}
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

	// 4. Request payment invoice from active gateway (PaymentG or multi-payment)
	gwID := req.PaymentGateway
	if gwID == "" && (req.PaymentMethod == "BINANCE_PAY" || req.PaymentMethod == "CRYPTO") {
		if req.PaymentChannel == "BEP20" || req.PaymentChannel == "USDT_BEP20" {
			gwID = "USDT_BEP20"
		} else {
			gwID = "BINANCE"
		}
	}
	gw, err := s.getPaymentGateway(gwID)
	if err != nil {
		if voucherClaimed && s.voucherService != nil {
			_ = s.voucherService.RevertVoucherUsage(appliedVoucherCode)
		}
		return nil, fmt.Errorf("gateway pembayaran tidak tersedia: %w", err)
	}

	metadata := fmt.Sprintf("Digimest: %s (Qty: %d)", targetProduct.Name, req.Quantity)
	payReq := &payment.CreatePaymentRequest{
		InvoiceNumber:   invoiceNumber,
		Amount:          subtotalIDR,
		ExpiryMinutes:   15,
		CustomerEmail:   req.CustomerEmail,
		CustomerContact: req.CustomerContact,
		Metadata:        metadata,
		Method:          payment.MethodQRIS,
		Channel:         "SHOPEEPAY",
	}
	if req.PaymentMethod != "" {
		payReq.Method = payment.Method(req.PaymentMethod)
	}
	if req.PaymentChannel != "" {
		payReq.Channel = req.PaymentChannel
	}

	// Calculate crypto amount if paying via Binance Pay or Crypto (BEP-20)
	if payReq.Method == payment.MethodBinancePay || payReq.Method == payment.MethodCrypto || gw.ID() == "BINANCE" || gw.ID() == "USDT_BEP20" {
		usdRate := s.pricing.GetUSDRate()
		if usdRate <= 0 {
			usdRate = 18000.0
		}
		amountUSDT := math.Round((float64(subtotalIDR)/usdRate)*100.0) / 100.0
		if amountUSDT < 0.01 {
			amountUSDT = 0.01
		}
		payReq.AmountCrypto = amountUSDT
		payReq.Currency = "USDT"
		if payReq.Channel == "" || payReq.Channel == "SHOPEEPAY" {
			if gw.ID() == "USDT_BEP20" {
				payReq.Channel = "BEP20"
			} else {
				payReq.Channel = "USDT"
			}
		}
	}

	payResp, err := gw.CreatePayment(context.Background(), payReq)
	if err != nil {
		// Revert voucher usage only if network gateway failure prevented payment generation entirely
		if voucherClaimed && s.voucherService != nil {
			_ = s.voucherService.RevertVoucherUsage(appliedVoucherCode)
		}
		return nil, fmt.Errorf("gagal membuat tagihan pembayaran (%s): %w", gw.Name(), err)
	}

	expiresAtTime := payResp.ExpiresAt
	if expiresAtTime == nil {
		t := time.Now().UTC().Add(15 * time.Minute)
		expiresAtTime = &t
	}

	// 5. Store order in DB
	order := &models.Order{
		InvoiceNumber:      invoiceNumber,
		ClaimCode:          claimCode,
		ProductSlug:        targetProduct.Slug,
		ProductName:        targetProduct.Name,
		ProductType:        targetProduct.Type,
		Quantity:           req.Quantity,
		BasePriceUSD:       unitPriceUSD,
		SellPriceIDR:       unitPriceIDR,
		OriginalAmount:     payResp.OriginalAmount,
		UniqueCode:         payResp.UniqueCode,
		TotalAmount:        payResp.TotalAmount,
		CryptoAmountStr:    payResp.CryptoAmountStr,
		CryptoCurrency:     payResp.CryptoCurrency,
		BinancePayID:       payResp.BinancePayID,
		BinanceAccountName: payResp.BinanceAccountName,
		WalletAddress:      payResp.WalletAddress,
		TokenContract:      payResp.TokenContract,
		Network:            payResp.Network,
		CexWarning:         payResp.CexWarning,
		VoucherCode:        appliedVoucherCode,
		DiscountIDR:        discountIDR,
		GatewayOrderID:     payResp.GatewayOrderID,
		PaymentGateway:     gw.ID(),
		PaymentMethod:      string(payResp.Method),
		PaymentChannel:     payResp.Channel,
		QRURL:              payResp.QRURL,
		PaymentStatus:      "PENDING",
		FulfillmentStatus:  "PENDING",
		WarrantyInfo:       targetProduct.Warranty,
		CustomerContact:    req.CustomerContact,
		CustomerEmail:      req.CustomerEmail,
		CostAtCheckoutUSD:  unitPriceUSD,
		ExpiresAt:          expiresAtTime,
	}

	if err := s.orderRepo.CreateOrder(order); err != nil {
		return nil, fmt.Errorf("gagal menyimpan pesanan ke database: %w", err)
	}

	return &models.CheckoutResponse{
		InvoiceNumber:      invoiceNumber,
		ClaimCode:          "", // Jangan bocorkan kode klaim sebelum lunas
		ProductName:        targetProduct.Name,
		Quantity:           req.Quantity,
		OriginalAmount:     payResp.OriginalAmount,
		UniqueCode:         payResp.UniqueCode,
		TotalAmount:        payResp.TotalAmount,
		CryptoAmountStr:    payResp.CryptoAmountStr,
		CryptoCurrency:     payResp.CryptoCurrency,
		BinancePayID:       payResp.BinancePayID,
		BinanceAccountName: payResp.BinanceAccountName,
		WalletAddress:      payResp.WalletAddress,
		TokenContract:      payResp.TokenContract,
		Network:            payResp.Network,
		CexWarning:         payResp.CexWarning,
		VoucherCode:        appliedVoucherCode,
		DiscountIDR:        discountIDR,
		GatewayOrderID:     payResp.GatewayOrderID,
		PaymentGateway:     gw.ID(),
		PaymentMethod:      string(payResp.Method),
		PaymentChannel:     payResp.Channel,
		QRURL:              payResp.QRURL,
		ExpiresAt:          expiresAtTime.Format(time.RFC3339),
		ExpiresInSeconds:   payResp.ExpiresInSeconds,
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

	// Sanitize BinanceTxID and TxHash to strictly prevent cross-contamination
	isBinance := (order.PaymentGateway == "BINANCE" || order.PaymentMethod == "BINANCE_PAY")
	isBEP20 := (order.PaymentGateway == "BEP20" || order.PaymentGateway == "USDT_BEP20" || order.PaymentMethod == "USDT" || order.PaymentMethod == "CRYPTO")

	cleanBinanceTxID := order.BinanceTxID
	if !isBinance || cleanBinanceTxID == "MANUAL_ADMIN_CONFIRMATION" || strings.HasPrefix(cleanBinanceTxID, "sbx_") {
		cleanBinanceTxID = ""
	}

	cleanTxHash := order.TxHash
	if !isBEP20 || !strings.HasPrefix(strings.ToLower(cleanTxHash), "0x") || len(cleanTxHash) < 60 {
		cleanTxHash = ""
	}

	res := &models.OrderStatusResponse{
		InvoiceNumber:      order.InvoiceNumber,
		ClaimCode:          visibleClaimCode,
		ProductName:        order.ProductName,
		ProductType:        order.ProductType,
		Quantity:           order.Quantity,
		TotalAmount:        order.TotalAmount,
		PaymentGateway:     order.PaymentGateway,
		PaymentMethod:      order.PaymentMethod,
		PaymentChannel:     order.PaymentChannel,
		CryptoAmountStr:    order.CryptoAmountStr,
		CryptoCurrency:     order.CryptoCurrency,
		BinanceTxID:        cleanBinanceTxID,
		BinancePayID:       order.BinancePayID,
		BinanceAccountName: order.BinanceAccountName,
		WalletAddress:      order.WalletAddress,
		TokenContract:      order.TokenContract,
		Network:            order.Network,
		TxHash:             cleanTxHash,
		CexWarning:         order.CexWarning,
		PaymentStatus:      order.PaymentStatus,
		FulfillmentStatus:  order.FulfillmentStatus,
		QRURL:              order.QRURL,
		DeliveredItems:     items,
		WarrantyInfo:       order.WarrantyInfo,
		CustomerContact:    order.CustomerContact,
		CustomerEmail:      order.CustomerEmail,
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
	} else if order.FulfillmentStatus == "PENDING_MANUAL_DELIVERY" {
		res.IsQuotaOut = true
		res.QuotaOutMessage = "Pembayaran Anda telah LUNAS! Lisensi pesanan ini sedang disiapkan manual oleh admin. Silakan hubungi Customer Service kami via Telegram untuk serah terima akun."
		res.DeliveredItems = []string{}
	} else if order.FulfillmentStatus == "HOLD_INSUFFICIENT_BOT_BALANCE" || order.FulfillmentStatus == "FAILED" {
		res.IsQuotaOut = true
		res.QuotaOutMessage = "Mohon maaf, terjadi kendala antrean pemenuhan stok otomatis dari penyedia server. Dana pembayaran Anda aman 100%. Silakan hubungi Customer Service via Telegram untuk penanganan kilat atau refund."
		res.DeliveredItems = []string{}
	}

	return res, nil
}

// CancelOrder cancels an unpaid pending order and returns voucher quota if used
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

	if err := s.orderRepo.UpdatePaymentStatus(order.InvoiceNumber, "CANCELLED", nil); err != nil {
		return err
	}

	// Revert voucher usage if order had a voucher applied
	if order.VoucherCode != "" && s.voucherService != nil {
		if revErr := s.voucherService.RevertVoucherUsage(order.VoucherCode); revErr != nil {
			log.Printf("[VOUCHER REVERT ERROR] Failed to revert voucher %s on cancel order %s: %v", order.VoucherCode, order.InvoiceNumber, revErr)
		} else {
			log.Printf("[VOUCHER REVERT SUCCESS] Reverted voucher %s on cancel order %s", order.VoucherCode, order.InvoiceNumber)
		}
	}

	return nil
}

// SweepExpiredOrders checks expired pending orders, reverts any claimed vouchers, and marks them EXPIRED
func (s *OrderService) SweepExpiredOrders() (int64, error) {
	expiredOrders, err := s.orderRepo.GetExpiredPendingOrders()
	if err != nil {
		log.Printf("[SWEEPER ERROR] Failed to query expired pending orders: %v", err)
	} else {
		for _, o := range expiredOrders {
			if o.VoucherCode != "" && s.voucherService != nil {
				if revErr := s.voucherService.RevertVoucherUsage(o.VoucherCode); revErr != nil {
					log.Printf("[VOUCHER SWEEPER ERROR] Failed to revert voucher %s on expired order %s: %v", o.VoucherCode, o.InvoiceNumber, revErr)
				} else {
					log.Printf("[VOUCHER SWEEPER SUCCESS] Reverted voucher %s on expired order %s", o.VoucherCode, o.InvoiceNumber)
				}
			}
		}
	}

	return s.orderRepo.SweepExpiredOrders()
}

// TriggerInstantCheck triggers immediate check on payment gateway and updates if paid
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

	gw, err := s.getPaymentGateway(order.PaymentGateway)
	if err != nil {
		return fmt.Errorf("gateway pembayaran '%s' tidak ditemukan: %w", order.PaymentGateway, err)
	}

	ctx := context.Background()

	// 1. Call instant mutation trigger on the gateway (if supported)
	_ = gw.TriggerInstantCheck(ctx, order.GatewayOrderID)

	// 2. Query gateway order status
	statusResp, err := gw.CheckStatus(ctx, order.GatewayOrderID)
	if err != nil {
		return err
	}

	if statusResp != nil && statusResp.Status == "PAID" {
		return s.ProcessPaymentSuccess(order.InvoiceNumber, statusResp.TransactionID)
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
		binanceTxID := ""
		txHash := ""

		isBinance := (order.PaymentGateway == "BINANCE" || order.PaymentMethod == "BINANCE_PAY")
		isBEP20 := (order.PaymentGateway == "BEP20" || order.PaymentGateway == "USDT_BEP20" || order.PaymentMethod == "USDT" || order.PaymentMethod == "CRYPTO")

		if isBinance {
			if txID != "" && txID != "MANUAL_ADMIN_CONFIRMATION" && !strings.HasPrefix(txID, "sbx_") {
				binanceTxID = txID
			}
		} else if isBEP20 {
			if strings.HasPrefix(strings.ToLower(txID), "0x") && len(txID) >= 60 {
				txHash = txID
			}
		}
		// For QRIS / PAYMENTG or MANUAL_ADMIN_CONFIRMATION, both binanceTxID and txHash remain strictly empty

		if err := s.orderRepo.UpdatePaymentSuccess(invoice, "PAID", &now, binanceTxID, txHash); err != nil {
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

	// LOCAL PRODUCT FULFILLMENT: Check if product is from local database
	if s.localProductRepo != nil {
		if lp, err := s.localProductRepo.GetBySlug(order.ProductSlug); err == nil && lp != nil {
			log.Printf("[LOCAL FULFILLMENT] Processing local product %s for order %s (Qty: %d)...", lp.Slug, invoice, order.Quantity)
			claimedKeys, claimErr := s.localProductRepo.ClaimKeys(lp.Slug, order.Quantity)
			if claimErr != nil {
				log.Printf("[LOCAL FULFILLMENT ERROR] Error claiming keys for %s: %v", invoice, claimErr)
			}

			if len(claimedKeys) > 0 {
				itemsJSON, _ := json.Marshal(claimedKeys)
				_ = s.orderRepo.UpdateFulfillmentWithCost(invoice, "SUCCESS", "", "LOCAL-ORDER", "", string(itemsJSON), 0)
				log.Printf("[LOCAL FULFILLMENT SUCCESS] Order %s fulfilled with %d key(s).", invoice, len(claimedKeys))
			} else {
				// Keys are empty. User requested: "Jika key kosong juga tidak apa-apa. Nanti user akan hubungi gue."
				_ = s.orderRepo.UpdateFulfillment(invoice, "PENDING_MANUAL_DELIVERY", "Stok serial key sedang disiapkan oleh admin. Hubungi Telegram CS kami di @Digimestsupport untuk penyerahan akun.", "", "", "[]")
				log.Printf("[LOCAL FULFILLMENT PENDING] Order %s marked PENDING_MANUAL_DELIVERY (empty keys).", invoice)
			}
			return nil
		}
	}

	// MULTI-PROVIDER FULFILLMENT: Check if provider registry is available
	if s.providerRegistry != nil {
		provID := s.resolveProviderID(order.ProductSlug)
		if prov, ok := s.providerRegistry.Get(provID); ok {
			ctx := context.Background()

			// PRE-FULFILLMENT CHECK: Check live wholesale cost from provider before buying
			liveCostUSD, err := prov.GetLiveCost(ctx, order.ProductSlug, order.Quantity)
			if err == nil && liveCostUSD > 0 {
				rate := s.pricing.GetUSDRate()
				liveTotalCostIDR := int64(math.Ceil(liveCostUSD * rate * float64(order.Quantity)))

				// Auto-hold conditions:
				// 1. Loss protection: liveTotalCostIDR > order.TotalAmount (selling below live modal cost)
				// 2. Upstream surge: liveCostUSD > base cost at checkout by >5%
				isSurge := false
				if order.BasePriceUSD > 0 && liveCostUSD > order.BasePriceUSD*1.05 {
					isSurge = true
				}
				if liveTotalCostIDR > order.TotalAmount || isSurge {
					errMsg := fmt.Sprintf("Lonjakan harga API upstream: modal naik menjadi Rp %d (uang bayar Rp %d). Pembelian otomatis ditahan demi perlindungan saldo bot.", liveTotalCostIDR, order.TotalAmount)
					log.Printf("[AUTO-HOLD PRICE SPIKE] %s: %s", invoice, errMsg)
					_ = s.orderRepo.UpdateFulfillment(invoice, "HOLD_PRICE_SPIKE", errMsg, "", "", "[]")
					return nil
				}

				reqUSD := liveCostUSD * float64(order.Quantity)
				if health, err := prov.CheckHealth(ctx); err == nil && health != nil && health.Configured {
					if health.Currency == "USD" && health.Balance < reqUSD {
						errMsg := fmt.Sprintf("Saldo bot %s ($%.2f) tidak mencukupi untuk pesanan ini ($%.2f). Pembelian otomatis ditahan untuk top-up saldo bot.", prov.Name(), health.Balance, reqUSD)
						log.Printf("[HOLD INSUFFICIENT BOT BALANCE] %s: %s", invoice, errMsg)
						_ = s.orderRepo.UpdateFulfillment(invoice, "HOLD_INSUFFICIENT_BOT_BALANCE", errMsg, "", "", "[]")
						return nil
					}
				}
			}

			log.Printf("[FULFILLMENT] Initiating auto-buy for %s via %s (Product: %s, Qty: %d)...", invoice, prov.Name(), order.ProductSlug, order.Quantity)
			fulReq := &provider.FulfillmentRequest{
				InvoiceNumber:   order.InvoiceNumber,
				ProductSlug:     order.ProductSlug,
				Quantity:        order.Quantity,
				CustomerEmail:   order.CustomerEmail,
				CustomerContact: order.CustomerContact,
				TotalAmountIDR:  order.TotalAmount,
			}
			fulRes, err := prov.Fulfill(ctx, fulReq)
			if err != nil {
				log.Printf("[FULFILLMENT ERROR] Order %s failed to buy from %s: %v", invoice, prov.Name(), err)
				_ = s.orderRepo.UpdateFulfillment(invoice, "FAILED", err.Error(), "", "", "[]")
				return nil
			}

			if !fulRes.Success {
				if fulRes.ErrorMessage == "PENDING_MANUAL_DELIVERY" {
					_ = s.orderRepo.UpdateFulfillmentWithCost(invoice, "PENDING_MANUAL_DELIVERY", "Pesanan item manual sedang diproses upstream. Lisensi akan diserahkan admin via Telegram.", fulRes.ProviderOrderID, fulRes.ProviderViewURL, "[]", fulRes.WholesaleCostUSD)
					log.Printf("[FULFILLMENT PENDING MANUAL] Order %s marked PENDING_MANUAL_DELIVERY from %s (Provider Order ID: %s)", invoice, prov.Name(), fulRes.ProviderOrderID)
					return nil
				}
				_ = s.orderRepo.UpdateFulfillment(invoice, "FAILED", fulRes.ErrorMessage, fulRes.ProviderOrderID, "", "[]")
				return nil
			}

			itemsJSON, _ := json.Marshal(fulRes.DeliveredItems)
			costUSD := liveCostUSD
			if fulRes.WholesaleCostUSD > 0 {
				costUSD = fulRes.WholesaleCostUSD
			}
			if err := s.orderRepo.UpdateFulfillmentWithCost(invoice, "SUCCESS", "", fulRes.ProviderOrderID, fulRes.ProviderViewURL, string(itemsJSON), costUSD); err != nil {
				log.Printf("Error updating fulfillment record for %s: %v", invoice, err)
			}

			log.Printf("[FULFILLMENT SUCCESS] Order %s fulfilled with %d item(s). Provider Order ID: %s", invoice, len(fulRes.DeliveredItems), fulRes.ProviderOrderID)
			return nil
		}
	}

	// LEGACY FALLBACK: Check live wholesale cost from DigitalCore before buying
	if s.dcClient != nil {
		liveCostUSD, err := s.dcClient.GetProductLiveCost(order.ProductSlug, order.Quantity)
		if err == nil && liveCostUSD > 0 {
			rate := s.pricing.GetUSDRate()
			liveTotalCostIDR := int64(math.Ceil(liveCostUSD * rate * float64(order.Quantity)))

			// Auto-hold conditions:
			// 1. Loss protection: liveTotalCostIDR > order.TotalAmount (selling below live modal cost)
			// 2. Upstream surge: liveCostUSD > base cost at checkout by >5%
			isSurge := false
			if order.BasePriceUSD > 0 && liveCostUSD > order.BasePriceUSD*1.05 {
				isSurge = true
			}
			if liveTotalCostIDR > order.TotalAmount || isSurge {
				errMsg := fmt.Sprintf("Lonjakan harga API upstream: modal naik menjadi Rp %d (uang bayar Rp %d). Pembelian otomatis ditahan demi perlindungan saldo bot.", liveTotalCostIDR, order.TotalAmount)
				log.Printf("[AUTO-HOLD PRICE SPIKE] %s: %s", invoice, errMsg)
				_ = s.orderRepo.UpdateFulfillment(invoice, "HOLD_PRICE_SPIKE", errMsg, "", "", "[]")
				return nil
			}

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

	if s.providerRegistry != nil {
		provID := s.resolveProviderID(order.ProductSlug)
		if prov, ok := s.providerRegistry.Get(provID); ok {
			log.Printf("[FORCE FULFILL] Admin forced buy for %s via provider %s...", invoice, prov.Name())
			fulReq := &provider.FulfillmentRequest{
				InvoiceNumber:   order.InvoiceNumber,
				ProductSlug:     order.ProductSlug,
				Quantity:        order.Quantity,
				CustomerEmail:   order.CustomerEmail,
				CustomerContact: order.CustomerContact,
				TotalAmountIDR:  order.TotalAmount,
			}
			fulRes, err := prov.Fulfill(context.Background(), fulReq)
			if err != nil {
				_ = s.orderRepo.UpdateFulfillment(invoice, "FAILED", err.Error(), "", "", "[]")
				return fmt.Errorf("gagal memenuhi stok dari %s: %w", prov.Name(), err)
			}

			if !fulRes.Success {
				if fulRes.ErrorMessage == "PENDING_MANUAL_DELIVERY" {
					_ = s.orderRepo.UpdateFulfillmentWithCost(invoice, "PENDING_MANUAL_DELIVERY", "Pesanan item manual sedang diproses upstream. Lisensi akan diserahkan admin via Telegram.", fulRes.ProviderOrderID, fulRes.ProviderViewURL, "[]", fulRes.WholesaleCostUSD)
					return nil
				}
				_ = s.orderRepo.UpdateFulfillment(invoice, "FAILED", fulRes.ErrorMessage, fulRes.ProviderOrderID, "", "[]")
				return fmt.Errorf("gagal memenuhi stok dari %s: %s", prov.Name(), fulRes.ErrorMessage)
			}

			itemsJSON, _ := json.Marshal(fulRes.DeliveredItems)
			costUSD := fulRes.WholesaleCostUSD
			if err := s.orderRepo.UpdateFulfillmentWithCost(invoice, "SUCCESS", "", fulRes.ProviderOrderID, fulRes.ProviderViewURL, string(itemsJSON), costUSD); err != nil {
				return err
			}
			return nil
		}
	}

	if s.dcClient != nil {
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

	return fmt.Errorf("tidak ada provider pemenuhan yang tersedia")
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

// ManualDeliverKeys allows admin to manually deliver serial keys for an order
func (s *OrderService) ManualDeliverKeys(invoice string, keys []string) error {
	lock := s.getInvoiceLock(invoice)
	lock.Lock()
	defer lock.Unlock()

	order, err := s.orderRepo.GetOrderByInvoice(invoice)
	if err != nil {
		return fmt.Errorf("pesanan '%s' tidak ditemukan", invoice)
	}

	if order.PaymentStatus != "PAID" {
		return fmt.Errorf("pesanan belum berstatus PAID")
	}

	var cleanKeys []string
	for _, k := range keys {
		trimmed := strings.TrimSpace(k)
		if trimmed != "" {
			cleanKeys = append(cleanKeys, trimmed)
		}
	}
	if len(cleanKeys) == 0 {
		return fmt.Errorf("minimal 1 serial key wajib diisi")
	}

	itemsJSON, _ := json.Marshal(cleanKeys)
	if err := s.orderRepo.UpdateFulfillmentWithCost(invoice, "SUCCESS", "", "MANUAL-DELIVERY", "", string(itemsJSON), 0); err != nil {
		return fmt.Errorf("gagal memperbarui status pengiriman: %w", err)
	}

	// Trigger email update
	if s.resendService != nil && order.CustomerEmail != "" {
		go func(toEmail, inv, claim string, total int64, prod string, qty int) {
			_ = s.resendService.SendClaimEmail(toEmail, inv, claim, total, prod, qty)
		}(order.CustomerEmail, order.InvoiceNumber, order.ClaimCode, order.TotalAmount, order.ProductName, order.Quantity)
	}

	return nil
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
	} else if order.FulfillmentStatus == "PENDING_MANUAL_DELIVERY" {
		isQuotaOut = true
		quotaOutMsg = "Pembayaran Anda telah LUNAS! Lisensi pesanan ini sedang disiapkan manual oleh admin. Silakan klik tombol di bawah untuk serah terima akun kilat dengan CS kami via Telegram."
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
