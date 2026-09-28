package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")
	ErrIdempotencyConflict   = errors.New("idempotency key reused with a different request")
)

type CreatePaymentRequest struct {
	AmountMinor int64             `json:"amount_minor"`
	Currency    string            `json:"currency"`
	CustomerRef string            `json:"customer_ref,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func ValidateIdempotencyKey(key string) error {
	if len(key) == 0 || len(key) > 255 || strings.TrimSpace(key) != key {
		return ErrInvalidIdempotencyKey
	}
	for _, char := range key {
		if char < 0x21 || char > 0x7e {
			return ErrInvalidIdempotencyKey
		}
	}
	return nil
}

func HashCreatePaymentRequest(request CreatePaymentRequest) ([sha256.Size]byte, error) {
	if request.AmountMinor <= 0 || !validCurrency(request.Currency) {
		return [sha256.Size]byte{}, ErrInvalidPayment
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("marshal idempotent request: %w", err)
	}
	return sha256.Sum256(payload), nil
}

func EnsureSameIdempotentRequest(stored, incoming [sha256.Size]byte) error {
	if subtle.ConstantTimeCompare(stored[:], incoming[:]) != 1 {
		return ErrIdempotencyConflict
	}
	return nil
}
