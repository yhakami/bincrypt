package models

import "time"

// CreateInvoiceRequest represents a request to create a BTCPay invoice.
type CreateInvoiceRequest struct {
	Amount            float64                `json:"amount"`
	Currency          string                 `json:"currency"`
	OrderID           string                 `json:"order_id,omitempty"`
	BuyerEmail        string                 `json:"buyer_email,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ExpirationMinutes int                    `json:"expiration_minutes"`
}

// CreateInvoiceResponse contains the response sent back to the client.
type CreateInvoiceResponse struct {
	InvoiceID   string    `json:"invoice_id"`
	CheckoutURL string    `json:"checkout_url"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
}

// InvoiceWebhookEvent is the minimal data BinCrypt needs from a BTCPay webhook.
type InvoiceWebhookEvent struct {
	Type          string                 `json:"type"`
	DeliveryID    string                 `json:"deliveryId"`
	WebhookID     string                 `json:"webhookId"`
	IsRedelivery  bool                   `json:"isRedelivery"`
	InvoiceID     string                 `json:"invoiceId"`
	StoreID       string                 `json:"storeId"`
	TimestampUnix int64                  `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata"`
}
