package service

import (
	"context"

	"storeku/internal/config"
	"storeku/internal/payment/paymentg"
)

// PaymentGOrderData alias for backward compatibility
type PaymentGOrderData = paymentg.OrderData

// PaymentGResponse alias for backward compatibility
type PaymentGResponse = paymentg.APIResponse

// PaymentGClient wraps paymentg.Client for backward compatibility
type PaymentGClient struct {
	cfg    *config.Config
	client *paymentg.Client
}

// NewPaymentGClient creates a new backward-compatible client wrapper
func NewPaymentGClient(cfg *config.Config) *PaymentGClient {
	return &PaymentGClient{
		cfg:    cfg,
		client: paymentg.NewClient(cfg),
	}
}

// CreateQRISOrder creates a dynamic QRIS order with PaymentG
func (c *PaymentGClient) CreateQRISOrder(invoice string, amount int64, metadata string) (*PaymentGOrderData, error) {
	return c.client.CreateQRISOrder(context.Background(), invoice, amount, metadata)
}

// CheckOrderStatus checks the status of an order on PaymentG
func (c *PaymentGClient) CheckOrderStatus(gatewayOrderID string) (*PaymentGOrderData, error) {
	return c.client.CheckOrderStatus(context.Background(), gatewayOrderID)
}

// TriggerInstantMutationCheck triggers an immediate ShopeePay mutation check
func (c *PaymentGClient) TriggerInstantMutationCheck(gatewayOrderID string) error {
	return c.client.TriggerInstantMutationCheck(context.Background(), gatewayOrderID)
}

// ResolveFullQRURL converts relative QR URL to absolute URL
func (c *PaymentGClient) ResolveFullQRURL(rawURL string) string {
	return c.client.ResolveFullQRURL(rawURL)
}

// Config returns the configuration
func (c *PaymentGClient) Config() *config.Config {
	return c.cfg
}

// Client returns the underlying paymentg.Client
func (c *PaymentGClient) Client() *paymentg.Client {
	return c.client
}
