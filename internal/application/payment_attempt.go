package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

var (
	ErrIntentNotFound             = errors.New("payment intent not found")
	ErrIntentNotPayable           = errors.New("payment intent is not payable")
	ErrAttemptProviderUnavailable = errors.New("payment provider unavailable")
)

type PaymentAttempt struct {
	ID              int64     `json:"id"`
	PaymentIntentID string    `json:"payment_intent_id"`
	AttemptNumber   int       `json:"attempt_number"`
	Status          string    `json:"status"`
	Provider        string    `json:"provider"`
	CreatedAt       time.Time `json:"created_at"`
}

type AttemptTransaction interface {
	PrepareAttempt(context.Context, int64, string, string, string) (PaymentAttempt, bool, int64, string, error)
	FinalizeAttempt(context.Context, int64, string, int64, string) (PaymentAttempt, error)
}

type AttemptStore interface {
	WithinAttemptTransaction(context.Context, func(AttemptTransaction) error) error
}

type CreatePaymentAttempt struct {
	store    AttemptStore
	provider PaymentProvider
}

func NewCreatePaymentAttempt(store AttemptStore, provider PaymentProvider) *CreatePaymentAttempt {
	return &CreatePaymentAttempt{store: store, provider: provider}
}

func (c *CreatePaymentAttempt) Execute(ctx context.Context, merchantID int64, intentID, key string) (PaymentAttempt, bool, error) {
	if c.store == nil || c.provider == nil || merchantID <= 0 || strings.TrimSpace(intentID) == "" {
		return PaymentAttempt{}, false, domain.ErrInvalidPayment
	}
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		return PaymentAttempt{}, false, err
	}
	var attempt PaymentAttempt
	var replay bool
	var amount int64
	var currency string
	providerName := strings.TrimSpace(c.provider.Name())
	if providerName == "" {
		return PaymentAttempt{}, false, domain.ErrInvalidPayment
	}
	err := c.store.WithinAttemptTransaction(ctx, func(tx AttemptTransaction) error {
		var err error
		attempt, replay, amount, currency, err = tx.PrepareAttempt(ctx, merchantID, intentID, key, providerName)
		return err
	})
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	if replay {
		return attempt, true, nil
	}
	result, err := c.provider.CreatePayment(ctx, ProviderRequest{PaymentIntentID: intentID, AttemptID: attempt.ID, AmountMinor: amount, Currency: currency, IdempotencyKey: fmt.Sprintf("payment-attempt-%d", attempt.ID)})
	if err != nil {
		return PaymentAttempt{}, false, fmt.Errorf("%w: %v", ErrAttemptProviderUnavailable, err)
	}
	if result.Status != ProviderAuthorized || strings.TrimSpace(result.Reference) == "" {
		return PaymentAttempt{}, false, ErrAttemptProviderUnavailable
	}
	err = c.store.WithinAttemptTransaction(ctx, func(tx AttemptTransaction) error {
		var err error
		attempt, err = tx.FinalizeAttempt(ctx, merchantID, intentID, attempt.ID, result.Reference)
		return err
	})
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	return attempt, false, nil
}
