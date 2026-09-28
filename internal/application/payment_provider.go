package application

import (
	"context"
	"errors"
)

var ErrInvalidProviderRequest = errors.New("invalid payment provider request")

type ProviderRequest struct {
	PaymentIntentID string
	AmountMinor     int64
	Currency        string
}

type ProviderStatus string

const ProviderAuthorized ProviderStatus = "AUTHORIZED"

type ProviderResult struct {
	Reference string
	Status    ProviderStatus
}

// PaymentProvider is the outbound port used to start a provider-side payment.
// It deliberately has no field for card numbers, CVV, or raw credentials.
type PaymentProvider interface {
	CreatePayment(context.Context, ProviderRequest) (ProviderResult, error)
}

type RefundProviderRequest struct {
	PaymentIntentID   string
	ProviderReference string
	AmountMinor       int64
	Currency          string
	IdempotencyKey    string
}

type RefundProviderResult struct{ Reference string }

type RefundProvider interface {
	Refund(context.Context, RefundProviderRequest) (RefundProviderResult, error)
}
