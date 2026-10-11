package domain

import (
	"errors"
	"testing"
)

func TestValidateIdempotencyKey(t *testing.T) {
	for _, key := range []string{"key-123", "request:2026-01"} {
		if err := ValidateIdempotencyKey(key); err != nil {
			t.Errorf("ValidateIdempotencyKey(%q): %v", key, err)
		}
	}
	for _, key := range []string{"", " leading", "trailing ", "bad\nkey"} {
		if err := ValidateIdempotencyKey(key); !errors.Is(err, ErrInvalidIdempotencyKey) {
			t.Errorf("ValidateIdempotencyKey(%q) error = %v", key, err)
		}
	}
}

func TestIdempotencyRequestHash(t *testing.T) {
	request := CreatePaymentRequest{AmountMinor: 1000, Currency: "IDR", Metadata: map[string]string{"b": "2", "a": "1"}}
	first, err := HashCreatePaymentRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashCreatePaymentRequest(CreatePaymentRequest{AmountMinor: 1000, Currency: "IDR", Metadata: map[string]string{"a": "1", "b": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureSameIdempotentRequest(first, second); err != nil {
		t.Fatalf("equivalent request hashes differ: %v", err)
	}
	changed, _ := HashCreatePaymentRequest(CreatePaymentRequest{AmountMinor: 2000, Currency: "IDR"})
	if err := EnsureSameIdempotentRequest(first, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("error = %v, want %v", err, ErrIdempotencyConflict)
	}
}
