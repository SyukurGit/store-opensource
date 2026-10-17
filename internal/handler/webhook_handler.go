package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"storeku/internal/payment"
	"storeku/internal/payment/paymentg"
	"storeku/internal/repository"
	"storeku/internal/service"
)

type WebhookHandler struct {
	orderService    *service.OrderService
	pgClient        *service.PaymentGClient
	orderRepo       *repository.OrderRepository
	paymentRegistry *payment.Registry
}

func NewWebhookHandler(orderService *service.OrderService, pgClient *service.PaymentGClient, orderRepo *repository.OrderRepository) *WebhookHandler {
	return &WebhookHandler{
		orderService: orderService,
		pgClient:     pgClient,
		orderRepo:    orderRepo,
	}
}

// SetPaymentRegistry injects the unified payment gateway registry
func (h *WebhookHandler) SetPaymentRegistry(pr *payment.Registry) {
	h.paymentRegistry = pr
}

func (h *WebhookHandler) getGateway(gatewayID string) payment.Gateway {
	normalizedID := strings.ToUpper(strings.TrimSpace(gatewayID))
	if h.paymentRegistry != nil {
		if gw, ok := h.paymentRegistry.Get(normalizedID); ok {
			return gw
		}
	}
	// Fallback to PaymentG adapter if requested or default
	if (normalizedID == "" || normalizedID == "PAYMENTG") && h.pgClient != nil {
		return paymentg.NewAdapterWithClient(h.pgClient.Config(), h.pgClient.Client())
	}
	return nil
}

// HandlePaymentGWebhook handles legacy POST /api/webhook/paymentg
func (h *WebhookHandler) HandlePaymentGWebhook(w http.ResponseWriter, r *http.Request) {
	gw := h.getGateway("PAYMENTG")
	if gw == nil {
		log.Printf("[WEBHOOK ERROR] PaymentG gateway adapter not configured")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ignored", "error": "gateway not configured"}`))
		return
	}

	notif, err := gw.VerifyWebhook(r.Context(), r)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] Failed to verify PaymentG webhook payload: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ignored", "error": "invalid payload"}`))
		return
	}

	if notif.Event == "payment.success" && notif.InvoiceNumber != "" {
		if !notif.IsVerified {
			log.Printf("[SECURITY ALERT] Webhook verification failed for invoice %s! Rejecting fulfillment.", notif.InvoiceNumber)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ignored", "error": "unverified payment"}`))
			return
		}

		log.Printf("[WEBHOOK RECEIVED] Gateway: PAYMENTG, Event: %s, Invoice: %s, TxID: %s, Amount: Rp %d",
			notif.Event, notif.InvoiceNumber, notif.TransactionID, notif.TotalAmount)

		if err := h.orderService.ProcessPaymentSuccess(notif.InvoiceNumber, notif.TransactionID); err != nil {
			log.Printf("[WEBHOOK PROCESSING ERROR] Order %s fulfillment error: %v", notif.InvoiceNumber, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}

// HandleGatewayWebhook handles generic POST /api/webhook/{gateway}
func (h *WebhookHandler) HandleGatewayWebhook(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gateway")
	if gatewayID == "" {
		if strings.Contains(r.URL.Path, "binance") {
			gatewayID = "BINANCE"
		} else if strings.Contains(r.URL.Path, "bep20") {
			gatewayID = "USDT_BEP20"
		} else {
			gatewayID = "PAYMENTG"
		}
	}

	gw := h.getGateway(gatewayID)
	if gw == nil {
		log.Printf("[WEBHOOK ERROR] Unknown payment gateway '%s'", gatewayID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status": "error", "message": "unknown payment gateway"}`))
		return
	}

	notif, err := gw.VerifyWebhook(r.Context(), r)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] Failed to verify webhook for gateway %s: %v", gw.ID(), err)
		w.Header().Set("Content-Type", "application/json")
		if gw.ID() == "BINANCE" || gw.ID() == "USDT_BEP20" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status": "unauthorized", "error": "` + err.Error() + `"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ignored", "error": "invalid payload"}`))
		}
		return
	}

	if notif.Event == "payment.success" && notif.InvoiceNumber != "" {
		if !notif.IsVerified {
			log.Printf("[SECURITY ALERT] Webhook verification failed for invoice %s via %s! Rejecting fulfillment.", notif.InvoiceNumber, gw.ID())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ignored", "error": "unverified payment"}`))
			return
		}

		log.Printf("[WEBHOOK RECEIVED] Gateway: %s, Event: %s, Invoice: %s, TxID: %s, Amount: Rp %d",
			gw.ID(), notif.Event, notif.InvoiceNumber, notif.TransactionID, notif.TotalAmount)

		if err := h.orderService.ProcessPaymentSuccess(notif.InvoiceNumber, notif.TransactionID); err != nil {
			log.Printf("[WEBHOOK PROCESSING ERROR] Order %s fulfillment error: %v", notif.InvoiceNumber, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "received"}`))
}
