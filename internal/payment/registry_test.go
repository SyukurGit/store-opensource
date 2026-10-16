package payment

import (
	"context"
	"net/http"
	"testing"
)

type mockGateway struct {
	id      string
	name    string
	methods []Method
}

func (m *mockGateway) ID() string                   { return m.id }
func (m *mockGateway) Name() string                 { return m.name }
func (m *mockGateway) SupportedMethods() []Method   { return m.methods }
func (m *mockGateway) CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*PaymentResponse, error) {
	return &PaymentResponse{GatewayOrderID: "MOCK-1", TotalAmount: req.Amount}, nil
}
func (m *mockGateway) CheckStatus(ctx context.Context, gatewayOrderID string) (*PaymentStatusResponse, error) {
	return &PaymentStatusResponse{GatewayOrderID: gatewayOrderID, Status: "PAID"}, nil
}
func (m *mockGateway) TriggerInstantCheck(ctx context.Context, gatewayOrderID string) error {
	return nil
}
func (m *mockGateway) VerifyWebhook(ctx context.Context, r *http.Request) (*WebhookNotification, error) {
	return &WebhookNotification{IsVerified: true, Event: "payment.success"}, nil
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Initial default should error
	if _, err := reg.Default(); err == nil {
		t.Fatalf("expected error from empty registry Default(), got nil")
	}

	gw1 := &mockGateway{id: "PAYMENTG", name: "PaymentG ShopeePay", methods: []Method{MethodQRIS}}
	gw2 := &mockGateway{id: "TRIPAY", name: "Tripay Gateway", methods: []Method{MethodQRIS, MethodVirtualAccount}}

	reg.Register(gw1)
	reg.Register(gw2)

	// Default should be gw1 (first registered)
	def, err := reg.Default()
	if err != nil {
		t.Fatalf("expected default gateway, got err: %v", err)
	}
	if def.ID() != "PAYMENTG" {
		t.Fatalf("expected default gateway PAYMENTG, got %s", def.ID())
	}

	// Change default
	if err := reg.SetDefault("TRIPAY"); err != nil {
		t.Fatalf("failed to set default to TRIPAY: %v", err)
	}
	def2, _ := reg.Default()
	if def2.ID() != "TRIPAY" {
		t.Fatalf("expected default TRIPAY, got %s", def2.ID())
	}

	// Get by ID case-insensitive
	g, ok := reg.Get("paymentg")
	if !ok || g.ID() != "PAYMENTG" {
		t.Fatalf("expected to get PAYMENTG, got ok=%v", ok)
	}

	// Get by Method
	vaGW, err := reg.GetByMethod(MethodVirtualAccount)
	if err != nil || vaGW.ID() != "TRIPAY" {
		t.Fatalf("expected TRIPAY for VirtualAccount, got: %v", vaGW)
	}

	// List
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 gateways in list, got %d", len(list))
	}
}
