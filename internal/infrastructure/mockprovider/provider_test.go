package mockprovider

import (
	"context"
	"errors"
	"testing"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

func TestCreatePayment(t *testing.T) {
	result, err := New().CreatePayment(context.Background(), application.ProviderRequest{
		PaymentIntentID: "pi_123",
		AttemptID:       1,
		IdempotencyKey:  "attempt-1",
		AmountMinor:     2500,
		Currency:        "IDR",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reference != "mock_pi_123_1" || result.Status != application.ProviderAuthorized {
		t.Fatalf("unexpected provider result: %+v", result)
	}
}

func TestCreatePaymentRejectsInvalidRequest(t *testing.T) {
	_, err := New().CreatePayment(context.Background(), application.ProviderRequest{PaymentIntentID: "pi_123", AttemptID: 1, IdempotencyKey: "attempt-1"})
	if !errors.Is(err, application.ErrInvalidProviderRequest) {
		t.Fatalf("error = %v, want %v", err, application.ErrInvalidProviderRequest)
	}
}

func TestRefundReturnsDeterministicReference(t *testing.T) {
	provider := New()
	request := application.RefundProviderRequest{PaymentIntentID: "pi_123", ProviderReference: "mock_pi_123", AmountMinor: 100, Currency: "IDR", IdempotencyKey: "refund-rf_123"}
	first, err := provider.Refund(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.Refund(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reference == "" || first != second {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}
