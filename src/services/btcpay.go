package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/models"
)

type InvoiceService interface {
	CreateInvoice(ctx context.Context, req *models.CreateInvoiceRequest) (*models.CreateInvoiceResponse, error)
	VerifyWebhook(ctx context.Context, body []byte, signatureHeader string) (*models.InvoiceWebhookEvent, error)
}

type BTCPayClient struct {
	endpoint      string
	storeID       string
	apiKey        string
	webhookSecret string
	httpClient    *http.Client
}

func NewBTCPayClient(endpoint, apiKey, storeID, webhookSecret string, httpClient *http.Client) (*BTCPayClient, error) {
	if endpoint == "" || apiKey == "" || storeID == "" {
		return nil, fmt.Errorf("btcpay configuration incomplete")
	}
	if webhookSecret == "" {
		return nil, fmt.Errorf("btcpay webhook secret is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	endpoint = strings.TrimRight(endpoint, "/")

	return &BTCPayClient{
		endpoint:      endpoint,
		storeID:       storeID,
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		httpClient:    httpClient,
	}, nil
}

func (c *BTCPayClient) CreateInvoice(ctx context.Context, req *models.CreateInvoiceRequest) (*models.CreateInvoiceResponse, error) {
	payload := map[string]interface{}{
		"amount":   req.Amount,
		"currency": strings.ToUpper(req.Currency),
	}

	metadata := map[string]interface{}{}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			metadata[k] = v
		}
	}
	if req.OrderID != "" {
		metadata["orderId"] = req.OrderID
	}
	if req.BuyerEmail != "" {
		metadata["buyerEmail"] = req.BuyerEmail
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}

	checkout := map[string]interface{}{}
	if req.ExpirationMinutes > 0 {
		checkout["expirationMinutes"] = req.ExpirationMinutes
	}
	if len(checkout) > 0 {
		payload["checkout"] = checkout
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invoice payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/stores/%s/invoices", c.endpoint, c.storeID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build btcpay request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "token "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("btcpay request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read btcpay response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("btcpay returned %d: %s", resp.StatusCode, truncate(string(respBody), 512))
	}

	var invoice struct {
		ID               string  `json:"id"`
		CheckoutLink     string  `json:"checkoutLink"`
		Amount           float64 `json:"amount"`
		Currency         string  `json:"currency"`
		Status           string  `json:"status"`
		AdditionalStatus string  `json:"additionalStatuses"`
		ExpiresAt        string  `json:"expiresAt"`
	}
	if err := json.Unmarshal(respBody, &invoice); err != nil {
		return nil, fmt.Errorf("failed to decode btcpay invoice response: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, invoice.ExpiresAt)
	if err != nil {
		expiresAt = time.Now().Add(15 * time.Minute)
	}

	return &models.CreateInvoiceResponse{
		InvoiceID:   invoice.ID,
		CheckoutURL: invoice.CheckoutLink,
		Status:      invoice.Status,
		ExpiresAt:   expiresAt,
		Amount:      invoice.Amount,
		Currency:    invoice.Currency,
	}, nil
}

func (c *BTCPayClient) VerifyWebhook(ctx context.Context, body []byte, signatureHeader string) (*models.InvoiceWebhookEvent, error) {
	if signatureHeader == "" {
		return nil, fmt.Errorf("missing btcpay signature header")
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return nil, fmt.Errorf("unexpected btcpay signature format")
	}
	signatureHex := strings.TrimPrefix(signatureHeader, prefix)
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return nil, fmt.Errorf("invalid btcpay signature encoding")
	}

	h := hmac.New(sha256.New, []byte(c.webhookSecret))
	h.Write(body)
	computed := h.Sum(nil)

	if !hmac.Equal(computed, signatureBytes) {
		return nil, fmt.Errorf("btcpay signature mismatch")
	}

	var event models.InvoiceWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to decode webhook payload: %w", err)
	}

	logger.Default().Info("btcpay_webhook_verified", logger.Fields{
		"invoice_id":  event.InvoiceID,
		"event_type":  event.Type,
		"store_id":    event.StoreID,
		"delivery_id": event.DeliveryID,
	})

	return &event, nil
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
