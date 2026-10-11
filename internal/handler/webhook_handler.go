package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"storeku/internal/repository"
	"storeku/internal/service"
)

type WebhookHandler struct {
	orderService *service.OrderService
	pgClient     *service.PaymentGClient
	orderRepo    *repository.OrderRepository
}

func NewWebhookHandler(orderService *service.OrderService, pgClient *service.PaymentGClient, orderRepo *repository.OrderRepository) *WebhookHandler {
	return &WebhookHandler{
		orderService: orderService,
		pgClient:     pgClient,
		orderRepo:    orderRepo,
	}
}

type PaymentGWebhookPayload struct {
	Event          string `json:"event"`
	OrderID        string `json:"order_id"`
	ReferenceID    string `json:"reference_id"`
	OriginalAmount int64  `json:"original_amount"`
	UniqueCode     int    `json:"unique_code"`
	TotalAmount    int64  `json:"total_amount"`
	PaidAt         string `json:"paid_at"`
	ShopeeTxID     string `json:"shopee_tx_id"`
}

// HandlePaymentGWebhook handles POST /api/webhook/paymentg
func (h *WebhookHandler) HandlePaymentGWebhook(w http.ResponseWriter, r *http.Request) {
	var payload PaymentGWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[WEBHOOK ERROR] Failed to decode webhook payload: %v", err)
		// Still return 200 to prevent retry storms on bad JSON payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ignored", "error": "invalid json"}`))
		return
	}

	log.Printf("[WEBHOOK RECEIVED] Event: %s, Invoice: %s, ShopeeTxID: %s, Amount: Rp %d",
		payload.Event, payload.ReferenceID, payload.ShopeeTxID, payload.TotalAmount)

	if payload.Event == "payment.success" && payload.ReferenceID != "" {
		// Double-check verification directly with PaymentG API server to prevent spoofed/fake webhooks
		if h.orderRepo != nil && h.pgClient != nil {
			order, err := h.orderRepo.GetOrderByInvoice(payload.ReferenceID)
			if err != nil || order == nil {
				log.Printf("[WEBHOOK WARNING] Invoice %s not found in local database. Ignoring.", payload.ReferenceID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status": "ignored", "error": "order not found"}`))
				return
			}

			if order.GatewayOrderID != "" {
				pgOrder, err := h.pgClient.CheckOrderStatus(order.GatewayOrderID)
				if err != nil || pgOrder == nil || pgOrder.Status != "PAID" {
					log.Printf("[SECURITY ALERT] Webhook verification failed for %s! PaymentG reported status: %v (err: %v). Rejecting fulfillment.", payload.ReferenceID, pgOrder, err)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"status": "ignored", "error": "unverified payment"}`))
					return
				}
			}
		}

		if err := h.orderService.ProcessPaymentSuccess(payload.ReferenceID, payload.ShopeeTxID); err != nil {
			log.Printf("[WEBHOOK PROCESSING ERROR] Order %s fulfillment error: %v", payload.ReferenceID, err)
		}
	}

	// PaymentG requires HTTP 200 OK with {"status": "ok"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}
