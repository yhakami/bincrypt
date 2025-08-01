package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bincrypt/src/models"
	"bincrypt/src/utils"
	"cloud.google.com/go/storage"
)

// InvoiceServiceInterface abstracts invoice operations for easier testing
type InvoiceServiceInterface interface {
	CreateInvoice(ctx context.Context, tier string) (*models.BTCPayInvoice, error)
	UpdateInvoiceStatus(ctx context.Context, invoiceID, status string) error
}

// InvoiceService handles BTCPay invoices
type InvoiceService struct {
	bucket       *storage.BucketHandle
	btcpayURL    string
	btcpayAPIKey string
	httpClient   *http.Client
}

// NewInvoiceService creates a new invoice service
func NewInvoiceService(bucket *storage.BucketHandle, btcpayURL, btcpayAPIKey string) *InvoiceService {
	return &InvoiceService{
		bucket:       bucket,
		btcpayURL:    strings.TrimRight(btcpayURL, "/"),
		btcpayAPIKey: btcpayAPIKey,
		httpClient:   &http.Client{Timeout: 20 * time.Second},
	}
}

// CreateInvoice creates a new BTCPay invoice
func (s *InvoiceService) CreateInvoice(ctx context.Context, tier string) (*models.BTCPayInvoice, error) {
	if tier != "premium" {
		return nil, fmt.Errorf("invalid tier: %s", tier)
	}

	// Generate user ID for tracking
	userID, err := utils.GenerateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user ID: %w", err)
	}

	// Create BTCPay invoice
	amount := 5.00
	invoiceReq := map[string]interface{}{
		"amount":   amount,
		"currency": "USD",
		"metadata": map[string]string{
			"tier":     "premium",
			"userId":   userID,
			"duration": "1", // 1 month
		},
	}

	body, _ := json.Marshal(invoiceReq)
	req, err := http.NewRequest("POST", s.btcpayURL+"/api/v1/invoices", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+s.btcpayAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("BTCPay error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var btcpayResp struct {
		ID           string `json:"id"`
		CheckoutLink string `json:"checkoutLink"`
		Status       string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&btcpayResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Store invoice in GCS
	invoice := &models.Invoice{
		ID:           btcpayResp.ID,
		UserID:       userID,
		Status:       btcpayResp.Status,
		AmountUSD:    amount,
		CheckoutLink: btcpayResp.CheckoutLink,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.saveInvoice(ctx, invoice); err != nil {
		// Log but don't fail - invoice is already created
		_ = err
	}

	return &models.BTCPayInvoice{
		ID:           btcpayResp.ID,
		CheckoutLink: btcpayResp.CheckoutLink,
		AmountUSD:    amount,
		Status:       btcpayResp.Status,
	}, nil
}

// UpdateInvoiceStatus updates invoice status from webhook
func (s *InvoiceService) UpdateInvoiceStatus(ctx context.Context, invoiceID, status string) error {
	// Load existing invoice
	invoice, err := s.getInvoice(ctx, invoiceID)
	if err != nil {
		// Create new record if not found
		invoice = &models.Invoice{
			ID:        invoiceID,
			Status:    status,
			CreatedAt: time.Now().UTC(),
		}
	}

	invoice.Status = status
	invoice.UpdatedAt = time.Now().UTC()

	return s.saveInvoice(ctx, invoice)
}

// saveInvoice saves invoice to GCS
func (s *InvoiceService) saveInvoice(ctx context.Context, invoice *models.Invoice) error {
	obj := s.bucket.Object(fmt.Sprintf("invoices/%s.json", invoice.ID))
	writer := obj.NewWriter(ctx)
	writer.ContentType = "application/json"

	if err := json.NewEncoder(writer).Encode(invoice); err != nil {
		writer.Close()
		return fmt.Errorf("failed to encode invoice: %w", err)
	}

	return writer.Close()
}

// getInvoice retrieves invoice from GCS
func (s *InvoiceService) getInvoice(ctx context.Context, invoiceID string) (*models.Invoice, error) {
	obj := s.bucket.Object(fmt.Sprintf("invoices/%s.json", invoiceID))
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var invoice models.Invoice
	if err := json.NewDecoder(reader).Decode(&invoice); err != nil {
		return nil, err
	}

	return &invoice, nil
}
