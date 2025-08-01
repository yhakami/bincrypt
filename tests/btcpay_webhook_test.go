package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"context"

	"bincrypt/src/handlers"
	"bincrypt/src/models"
)

type stubInvoiceService struct{}

func (s *stubInvoiceService) CreateInvoice(ctx context.Context, tier string) (*models.BTCPayInvoice, error) {
	return nil, nil
}

func (s *stubInvoiceService) UpdateInvoiceStatus(ctx context.Context, invoiceID, status string) error {
	return nil
}

func TestPaymentWebhookSignature(t *testing.T) {
	h := handlers.NewInvoiceHandler(&stubInvoiceService{})
	body := []byte(`{"invoiceId":"123","status":"Confirmed"}`)
	nonce := "nonce"
	secret := "testsecret"
	os.Setenv("BTCPAY_WEBHOOK_SECRET", secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(nonce))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("BTCPay-Sig", sig)
	req.Header.Set("BTCPay-Nonce", nonce)
	w := httptest.NewRecorder()
	h.PaymentWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPaymentWebhookInvalidSig(t *testing.T) {
	h := handlers.NewInvoiceHandler(&stubInvoiceService{})
	body := []byte(`{"invoiceId":"123","status":"Confirmed"}`)
	os.Setenv("BTCPAY_WEBHOOK_SECRET", "testsecret")
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("BTCPay-Sig", "sha256=bad")
	req.Header.Set("BTCPay-Nonce", "n")
	w := httptest.NewRecorder()
	h.PaymentWebhook(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPaymentWebhookMissingSig(t *testing.T) {
	h := handlers.NewInvoiceHandler(&stubInvoiceService{})
	body := []byte(`{"invoiceId":"123","status":"Confirmed"}`)
	os.Setenv("BTCPAY_WEBHOOK_SECRET", "testsecret")
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.PaymentWebhook(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
