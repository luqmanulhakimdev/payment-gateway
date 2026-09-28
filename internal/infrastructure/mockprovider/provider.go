package mockprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

// Provider simulates a provider accepting a payment without handling card data.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) Name() string { return "mock" }

func (*Provider) CreatePayment(_ context.Context, request application.ProviderRequest) (application.ProviderResult, error) {
	if strings.TrimSpace(request.PaymentIntentID) == "" || request.AttemptID <= 0 || request.IdempotencyKey == "" || request.AmountMinor <= 0 || !validCurrency(request.Currency) {
		return application.ProviderResult{}, application.ErrInvalidProviderRequest
	}
	return application.ProviderResult{
		Reference: fmt.Sprintf("mock_%s_%d", request.PaymentIntentID, request.AttemptID),
		Status:    application.ProviderAuthorized,
	}, nil
}

func (*Provider) LookupPayment(_ context.Context, request application.ProviderRequest) (application.ProviderResult, error) {
	if strings.TrimSpace(request.PaymentIntentID) == "" || request.AttemptID <= 0 || request.IdempotencyKey != fmt.Sprintf("payment-attempt-%d", request.AttemptID) || request.AmountMinor <= 0 || !validCurrency(request.Currency) {
		return application.ProviderResult{}, application.ErrInvalidProviderRequest
	}
	// The deterministic adapter models a provider that accepted the request even if the
	// original HTTP response was lost. Real adapters query their provider by idempotency key.
	return application.ProviderResult{Reference: fmt.Sprintf("mock_%s_%d", request.PaymentIntentID, request.AttemptID), Status: application.ProviderAuthorized}, nil
}

func (*Provider) Refund(_ context.Context, request application.RefundProviderRequest) (application.RefundProviderResult, error) {
	if strings.TrimSpace(request.PaymentIntentID) == "" || strings.TrimSpace(request.ProviderReference) == "" || request.AmountMinor <= 0 || !validCurrency(request.Currency) || request.IdempotencyKey == "" {
		return application.RefundProviderResult{}, application.ErrInvalidProviderRequest
	}
	digest := sha256.Sum256([]byte(request.IdempotencyKey))
	return application.RefundProviderResult{Reference: "mock_rf_" + hex.EncodeToString(digest[:12])}, nil
}

var _ application.RefundProvider = (*Provider)(nil)

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, char := range currency {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}
