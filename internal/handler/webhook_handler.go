package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"storeku/internal/service"
)

type WebhookHandler struct {
	orderService *service.OrderService
}

func NewWebhookHandler(orderService *service.OrderService) *WebhookHandler {
	return &WebhookHandler{
		orderService: orderService,
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
		if err := h.orderService.ProcessPaymentSuccess(payload.ReferenceID, payload.ShopeeTxID); err != nil {
			log.Printf("[WEBHOOK PROCESSING ERROR] Order %s fulfillment error: %v", payload.ReferenceID, err)
		}
	}

	// PaymentG requires HTTP 200 OK with {"status": "ok"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}
