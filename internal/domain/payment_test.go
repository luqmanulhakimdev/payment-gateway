package domain

import (
	"errors"
	"testing"
)

func validIntent(status PaymentStatus) PaymentIntent {
	return PaymentIntent{ID: "pi_123", MerchantID: 7, AmountMinor: 2500, Currency: "IDR", Status: status}
}

func TestPaymentTransitions(t *testing.T) {
	tests := []struct {
		from    PaymentStatus
		to      PaymentStatus
		wantErr error
	}{
		{PaymentCreated, PaymentPending, nil},
		{PaymentPending, PaymentAuthorized, nil},
		{PaymentPending, PaymentPaid, nil},
		{PaymentAuthorized, PaymentPaid, nil},
		{PaymentPaid, PaymentRefunded, nil},
		{PaymentFailed, PaymentPaid, ErrInvalidTransition},
		{PaymentRefunded, PaymentPaid, ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			got, err := validIntent(tt.from).Transition(tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Status != tt.to {
				t.Fatalf("status = %s, want %s", got.Status, tt.to)
			}
		})
	}
}

func TestApplyRefund(t *testing.T) {
	tests := []struct {
		name    string
		intent  PaymentIntent
		refund  int64
		already int64
		want    PaymentStatus
		wantErr error
	}{
		{"partial refund retains paid state", validIntent(PaymentPaid), 500, 0, PaymentPaid, nil},
		{"final refund marks intent refunded", validIntent(PaymentPaid), 500, 2000, PaymentRefunded, nil},
		{"refund over captured amount fails", validIntent(PaymentPaid), 1, 2500, "", ErrRefundExceedsAmount},
		{"unpaid intent cannot be refunded", validIntent(PaymentPending), 1, 0, "", ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyRefund(tt.intent, tt.refund, tt.already)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Status != tt.want {
				t.Fatalf("status = %s, want %s", got.Status, tt.want)
			}
		})
	}
}
