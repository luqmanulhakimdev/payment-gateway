package mockprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

// Provider simulates a provider accepting a payment without handling card data.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) CreatePayment(_ context.Context, request application.ProviderRequest) (application.ProviderResult, error) {
	if strings.TrimSpace(request.PaymentIntentID) == "" || request.AmountMinor <= 0 || !validCurrency(request.Currency) {
		return application.ProviderResult{}, application.ErrInvalidProviderRequest
	}
	return application.ProviderResult{
		Reference: fmt.Sprintf("mock_%s", request.PaymentIntentID),
		Status:    application.ProviderAuthorized,
	}, nil
}

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
