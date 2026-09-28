package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

var (
	ErrInvalidMerchant  = errors.New("invalid merchant")
	ErrCustomerNotFound = errors.New("customer not found for merchant")
)

type IntentTransaction interface {
	ReserveIdempotency(context.Context, int64, string, [32]byte, time.Time) (IdempotencyReservation, error)
	CreateIntent(context.Context, IntentRecord) (CreatedIntent, error)
	CompleteIdempotency(context.Context, int64, string, int, []byte, int64) error
}

type IntentStore interface {
	WithinTransaction(context.Context, func(IntentTransaction) error) error
}

type IdempotencyReservation struct {
	Replayed     bool
	ResponseCode int
	ResponseBody []byte
}

type IntentRecord struct {
	PublicID    string
	MerchantID  int64
	CustomerRef string
	AmountMinor int64
	Currency    string
	Description string
	Metadata    map[string]string
}

type CreatedIntent struct {
	DatabaseID  int64     `json:"-"`
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateIntentResult struct {
	StatusCode int
	Body       []byte
	Replayed   bool
}

type CreateIntent struct {
	store IntentStore
	now   func() time.Time
}

func NewCreateIntent(store IntentStore) *CreateIntent {
	return &CreateIntent{store: store, now: time.Now}
}

func (c *CreateIntent) Execute(ctx context.Context, merchantID int64, key string, request domain.CreatePaymentRequest) (CreateIntentResult, error) {
	if c.store == nil || merchantID <= 0 {
		return CreateIntentResult{}, ErrInvalidMerchant
	}
	if err := domain.ValidateIdempotencyKey(key); err != nil {
		return CreateIntentResult{}, err
	}
	requestHash, err := domain.HashCreatePaymentRequest(request)
	if err != nil {
		return CreateIntentResult{}, err
	}
	publicID, err := newPaymentIntentID()
	if err != nil {
		return CreateIntentResult{}, fmt.Errorf("generate payment intent ID: %w", err)
	}

	var result CreateIntentResult
	err = c.store.WithinTransaction(ctx, func(tx IntentTransaction) error {
		reservation, err := tx.ReserveIdempotency(ctx, merchantID, key, requestHash, c.now().Add(24*time.Hour))
		if err != nil {
			return err
		}
		if reservation.Replayed {
			result = CreateIntentResult{StatusCode: reservation.ResponseCode, Body: append([]byte(nil), reservation.ResponseBody...), Replayed: true}
			return nil
		}
		intent, err := tx.CreateIntent(ctx, IntentRecord{
			PublicID: publicID, MerchantID: merchantID, CustomerRef: request.CustomerRef,
			AmountMinor: request.AmountMinor, Currency: request.Currency,
			Description: request.Description, Metadata: request.Metadata,
		})
		if err != nil {
			return err
		}
		body, err := json.Marshal(intent)
		if err != nil {
			return fmt.Errorf("encode payment intent: %w", err)
		}
		if err := tx.CompleteIdempotency(ctx, merchantID, key, 201, body, intent.DatabaseID); err != nil {
			return err
		}
		result = CreateIntentResult{StatusCode: 201, Body: body}
		return nil
	})
	if err != nil {
		return CreateIntentResult{}, err
	}
	return result, nil
}

func newPaymentIntentID() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "pi_" + hex.EncodeToString(entropy[:]), nil
}
