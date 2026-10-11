package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

var (
	ErrRefundNotFound            = errors.New("paid payment intent not found")
	ErrRefundConflict            = errors.New("refund idempotency key conflicts with request")
	ErrRefundInProgress          = errors.New("refund is already in progress")
	ErrRefundProviderUnavailable = errors.New("refund provider unavailable")
)

type Refund struct {
	ID                string    `json:"id"`
	PaymentIntentID   string    `json:"payment_intent_id"`
	AmountMinor       int64     `json:"amount_minor"`
	Currency          string    `json:"currency"`
	Status            string    `json:"status"`
	Reason            string    `json:"reason,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	ProviderReference string    `json:"-"`
}

type RefundRequest struct {
	AmountMinor int64  `json:"amount_minor"`
	Reason      string `json:"reason"`
}

type RefundTransaction interface {
	PrepareRefund(context.Context, int64, string, string, string, [32]byte, RefundRequest) (Refund, bool, error)
	FinalizeRefund(context.Context, int64, string, string, string) (Refund, error)
}

type RefundStore interface {
	WithinRefundTransaction(context.Context, func(RefundTransaction) error) error
}

type CreateRefund struct {
	store    RefundStore
	provider RefundProvider
}

func NewCreateRefund(store RefundStore, provider RefundProvider) *CreateRefund {
	return &CreateRefund{store: store, provider: provider}
}

func (c *CreateRefund) Execute(ctx context.Context, merchantID int64, intentID, key string, request RefundRequest) (Refund, bool, error) {
	if c.store == nil || c.provider == nil || merchantID <= 0 || intentID == "" || request.AmountMinor <= 0 || len(request.Reason) > 500 {
		return Refund{}, false, domain.ErrInvalidPayment
	}
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		return Refund{}, false, err
	}
	hashBytes, err := json.Marshal(request)
	if err != nil {
		return Refund{}, false, err
	}
	hash := sha256.Sum256(hashBytes)
	publicID, err := newRefundID()
	if err != nil {
		return Refund{}, false, fmt.Errorf("generate refund ID: %w", err)
	}
	var refund Refund
	var replay bool
	err = c.store.WithinRefundTransaction(ctx, func(tx RefundTransaction) error {
		var err error
		refund, replay, err = tx.PrepareRefund(ctx, merchantID, intentID, key, publicID, hash, request)
		return err
	})
	if err != nil {
		return Refund{}, false, err
	}
	if replay {
		return refund, true, nil
	}
	if refund.Status != "PENDING" {
		return Refund{}, false, ErrRefundInProgress
	}
	providerResult, err := c.provider.Refund(ctx, RefundProviderRequest{
		PaymentIntentID: intentID, ProviderReference: refundProviderRef(refund), AmountMinor: request.AmountMinor,
		Currency: refund.Currency, IdempotencyKey: "refund-" + refund.ID,
	})
	if err != nil {
		return Refund{}, false, fmt.Errorf("%w: %v", ErrRefundProviderUnavailable, err)
	}
	if providerResult.Reference == "" {
		return Refund{}, false, ErrRefundProviderUnavailable
	}
	var finalized Refund
	err = c.store.WithinRefundTransaction(ctx, func(tx RefundTransaction) error {
		var err error
		finalized, err = tx.FinalizeRefund(ctx, merchantID, intentID, refund.ID, providerResult.Reference)
		return err
	})
	if err != nil {
		return Refund{}, false, err
	}
	return finalized, false, nil
}

func newRefundID() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "rf_" + hex.EncodeToString(entropy[:]), nil
}

// Provider reference is an internal persistence detail populated by the store.
func refundProviderRef(refund Refund) string { return refund.ProviderReference }
