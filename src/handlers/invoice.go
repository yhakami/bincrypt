package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"bincrypt/src/models"
	"bincrypt/src/services"
)

// InvoiceHandler handles invoice operations
type InvoiceHandler struct {
	invoiceService *services.InvoiceService
}

// NewInvoiceHandler creates a new invoice handler
func NewInvoiceHandler(invoiceService *services.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService}
}

// CreateInvoice handles invoice creation
func (h *InvoiceHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB max
	
	var req models.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	ctx := r.Context()
	invoice, err := h.invoiceService.CreateInvoice(ctx, req.Tier)
	if err != nil {
		log.Printf("Failed to create invoice: %v", err)
		http.Error(w, "Failed to create invoice", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

// PaymentWebhook handles BTCPay webhooks
func (h *InvoiceHandler) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	// Read body for signature verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	// Verify signature if secret is set
	secret := os.Getenv("BTCPAY_WEBHOOK_SECRET")
	if secret != "" {
		sig := r.Header.Get("BTCPay-Sig")
		if sig == "" {
			http.Error(w, "Missing webhook signature", http.StatusUnauthorized)
			return
		}
		
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		
		if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
			http.Error(w, "Invalid webhook signature", http.StatusUnauthorized)
			return
		}
	}
	
	// Parse webhook
	var webhook map[string]interface{}
	if err := json.Unmarshal(body, &webhook); err != nil {
		http.Error(w, "Invalid webhook", http.StatusBadRequest)
		return
	}
	
	// Extract invoice ID and status
	invoiceID, _ := webhook["invoiceId"].(string)
	status, _ := webhook["status"].(string)
	
	if invoiceID == "" || status == "" {
		http.Error(w, "Missing invoiceId or status", http.StatusBadRequest)
		return
	}
	
	// Update invoice status
	ctx := r.Context()
	if err := h.invoiceService.UpdateInvoiceStatus(ctx, invoiceID, status); err != nil {
		log.Printf("Failed to update invoice status: %v", err)
		// Don't fail webhook - BTCPay will retry
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}