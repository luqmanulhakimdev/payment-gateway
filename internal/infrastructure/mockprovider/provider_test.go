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
		AmountMinor:     2500,
		Currency:        "IDR",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reference != "mock_pi_123" || result.Status != application.ProviderAuthorized {
		t.Fatalf("unexpected provider result: %+v", result)
	}
}

func TestCreatePaymentRejectsInvalidRequest(t *testing.T) {
	_, err := New().CreatePayment(context.Background(), application.ProviderRequest{PaymentIntentID: "pi_123"})
	if !errors.Is(err, application.ErrInvalidProviderRequest) {
		t.Fatalf("error = %v, want %v", err, application.ErrInvalidProviderRequest)
	}
}
