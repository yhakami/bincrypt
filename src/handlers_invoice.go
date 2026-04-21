package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/models"
	"github.com/yhakami/bincrypt/src/utils"
)

const (
	defaultInvoiceExpirationMinutes = 30
	minInvoiceExpirationMinutes     = 5
	maxInvoiceExpirationMinutes     = 24 * 60
	maxInvoiceMetadataBytes         = 2048
	maxWebhookBodyBytes             = 1 << 20
)

func (s *Server) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	if s.invoiceService == nil {
		WriteSimpleError(w, "Payments unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	log := logger.WithContext(ctx)
	audit := logger.GetAuditLogger()
	clientIP := utils.GetClientIP(r, s.proxyConfig)

	var req models.CreateInvoiceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		log.Warn("invoice_request_invalid", logger.Fields{"error": err.Error(), "client_ip": clientIP})
		WriteSimpleError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	if req.ExpirationMinutes == 0 {
		req.ExpirationMinutes = defaultInvoiceExpirationMinutes
	}
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))

	if errs := validateCreateInvoiceRequest(&req); len(errs) > 0 {
		log.Warn("invoice_request_validation_failed", logger.Fields{"client_ip": clientIP, "errors": errs})
		WriteValidationError(w, errs)
		return
	}

	invoice, err := s.invoiceService.CreateInvoice(ctx, &req)
	if err != nil {
		log.Error("invoice_create_failed", logger.Fields{"error": err.Error(), "client_ip": clientIP})
		WriteSimpleError(w, "Failed to create invoice", http.StatusBadGateway)
		return
	}

	audit.LogInvoiceCreated(ctx, invoice.InvoiceID, req.Amount, req.Currency, clientIP)
	log.Info("invoice_created", logger.Fields{"invoice_id": invoice.InvoiceID, "amount": req.Amount, "currency": req.Currency, "client_ip": clientIP})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	_ = json.NewEncoder(w).Encode(invoice)
}

func (s *Server) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if s.invoiceService == nil {
		WriteSimpleError(w, "Payments unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	log := logger.WithContext(ctx)
	audit := logger.GetAuditLogger()
	clientIP := utils.GetClientIP(r, s.proxyConfig)

	signature := r.Header.Get("BTCPay-Sig")
	if signature == "" {
		signature = r.Header.Get("BTCPAY-SIG")
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes))
	if err != nil {
		log.Error("webhook_body_read_failed", logger.Fields{"error": err.Error()})
		WriteSimpleError(w, "Failed to read webhook", http.StatusInternalServerError)
		return
	}

	event, err := s.invoiceService.VerifyWebhook(ctx, body, signature)
	if err != nil {
		log.Warn("webhook_verification_failed", logger.Fields{"error": err.Error(), "client_ip": clientIP})
		audit.LogWebhookReceived(ctx, "btcpay", false, clientIP, signature)
		WriteSimpleError(w, "Invalid webhook", http.StatusBadRequest)
		return
	}

	audit.LogWebhookReceived(ctx, event.Type, true, clientIP, signature)
	log.Info("btcpay_webhook_received", logger.Fields{"invoice_id": event.InvoiceID, "event_type": event.Type, "store_id": event.StoreID})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
