package payment

import (
	"context"
	"net/http"
	"time"
)

// Method represents standard payment methods supported across gateways
type Method string

const (
	MethodQRIS           Method = "QRIS"
	MethodVirtualAccount Method = "VIRTUAL_ACCOUNT"
	MethodEWallet        Method = "EWALLET"
	MethodRetailOutlet   Method = "RETAIL_OUTLET"
	MethodManualTransfer Method = "MANUAL_TRANSFER"
)

// CreatePaymentRequest defines the standardized input to create a payment on any gateway
type CreatePaymentRequest struct {
	InvoiceNumber   string
	Amount          int64
	ExpiryMinutes   int
	CustomerEmail   string
	CustomerContact string
	Metadata        string
	Method          Method
	Channel         string // e.g. "SHOPEEPAY", "BCA", "GOPAY", "OVO"
}

// PaymentResponse is the standardized output returned by any gateway
type PaymentResponse struct {
	GatewayOrderID   string
	GatewayID        string
	Method           Method
	Channel          string
	OriginalAmount   int64
	UniqueCode       int
	TotalAmount      int64
	QRURL            string
	PaymentURL       string
	ExpiresAt        *time.Time
	ExpiresInSeconds int
	RawData          map[string]interface{}
}

// PaymentStatusResponse represents standardized status returned when querying gateway
type PaymentStatusResponse struct {
	GatewayOrderID string
	GatewayID      string
	Status         string // "PENDING", "PAID", "EXPIRED", "CANCELLED", "FAILED"
	PaidAt         *time.Time
	TransactionID  string
	TotalAmount    int64
	RawStatus      string
}

// WebhookNotification represents parsed and verified webhook data
type WebhookNotification struct {
	Event          string // e.g. "payment.success"
	GatewayID      string
	InvoiceNumber  string
	GatewayOrderID string
	TransactionID  string
	TotalAmount    int64
	PaidAt         *time.Time
	IsVerified     bool
	RawPayload     []byte
}

// Gateway is the primary interface every payment provider must satisfy
type Gateway interface {
	// ID returns the unique gateway identifier, e.g. "PAYMENTG", "TRIPAY", "MIDTRANS"
	ID() string

	// Name returns the human-readable display name of the gateway
	Name() string

	// SupportedMethods returns payment methods supported by this gateway
	SupportedMethods() []Method

	// CreatePayment requests a new payment transaction with the gateway
	CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*PaymentResponse, error)

	// CheckStatus checks the current status of an order on the gateway
	CheckStatus(ctx context.Context, gatewayOrderID string) (*PaymentStatusResponse, error)

	// TriggerInstantCheck triggers an immediate mutation verification (e.g. "Saya Sudah Bayar")
	TriggerInstantCheck(ctx context.Context, gatewayOrderID string) error

	// VerifyWebhook parses and verifies an incoming HTTP webhook callback from the gateway
	VerifyWebhook(ctx context.Context, r *http.Request) (*WebhookNotification, error)
}
