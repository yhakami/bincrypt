package models

import "time"

// CreateInvoiceRequest represents an invoice creation request
type CreateInvoiceRequest struct {
	Tier string `json:"tier"`
}

// Invoice represents a payment invoice
type Invoice struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Status       string    `json:"status"`
	AmountUSD    float64   `json:"amount_usd"`
	CheckoutLink string    `json:"checkout_link,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BTCPayInvoice represents the public invoice response
type BTCPayInvoice struct {
	ID           string  `json:"id"`
	CheckoutLink string  `json:"checkout_link"`
	AmountUSD    float64 `json:"amount_usd"`
	Status       string  `json:"status"`
}