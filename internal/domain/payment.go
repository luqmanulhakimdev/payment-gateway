package domain

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidPayment      = errors.New("invalid payment")
	ErrInvalidTransition   = errors.New("invalid payment state transition")
	ErrRefundExceedsAmount = errors.New("refund exceeds captured amount")
)

type PaymentStatus string

const (
	PaymentCreated    PaymentStatus = "CREATED"
	PaymentPending    PaymentStatus = "PENDING"
	PaymentAuthorized PaymentStatus = "AUTHORIZED"
	PaymentPaid       PaymentStatus = "PAID"
	PaymentFailed     PaymentStatus = "FAILED"
	PaymentExpired    PaymentStatus = "EXPIRED"
	PaymentCancelled  PaymentStatus = "CANCELLED"
	PaymentRefunded   PaymentStatus = "REFUNDED"
)

type PaymentIntent struct {
	ID          string
	MerchantID  int64
	AmountMinor int64
	Currency    string
	Status      PaymentStatus
}

func (p PaymentIntent) Validate() error {
	if strings.TrimSpace(p.ID) == "" || p.MerchantID <= 0 || p.AmountMinor <= 0 || !validCurrency(p.Currency) {
		return ErrInvalidPayment
	}
	if !validStatus(p.Status) {
		return fmt.Errorf("%w: unknown status %q", ErrInvalidPayment, p.Status)
	}
	return nil
}

func (p PaymentIntent) Transition(to PaymentStatus) (PaymentIntent, error) {
	if err := p.Validate(); err != nil {
		return PaymentIntent{}, err
	}
	allowed := map[PaymentStatus][]PaymentStatus{
		PaymentCreated:    {PaymentPending, PaymentCancelled, PaymentExpired},
		PaymentPending:    {PaymentAuthorized, PaymentPaid, PaymentFailed, PaymentExpired, PaymentCancelled},
		PaymentAuthorized: {PaymentPaid, PaymentFailed, PaymentCancelled},
		PaymentPaid:       {PaymentRefunded},
	}
	for _, next := range allowed[p.Status] {
		if next == to {
			p.Status = to
			return p, nil
		}
	}
	return PaymentIntent{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, p.Status, to)
}

// ApplyRefund marks the intent refunded only after its full captured amount is returned.
func ApplyRefund(p PaymentIntent, refundMinor, alreadyRefundedMinor int64) (PaymentIntent, error) {
	if err := p.Validate(); err != nil {
		return PaymentIntent{}, err
	}
	if p.Status != PaymentPaid && p.Status != PaymentRefunded {
		return PaymentIntent{}, fmt.Errorf("%w: refunds require a paid intent", ErrInvalidTransition)
	}
	if refundMinor <= 0 || alreadyRefundedMinor < 0 || alreadyRefundedMinor > p.AmountMinor || refundMinor > p.AmountMinor-alreadyRefundedMinor {
		return PaymentIntent{}, ErrRefundExceedsAmount
	}
	if refundMinor == p.AmountMinor-alreadyRefundedMinor {
		p.Status = PaymentRefunded
	}
	return p, nil
}

func validStatus(status PaymentStatus) bool {
	switch status {
	case PaymentCreated, PaymentPending, PaymentAuthorized, PaymentPaid, PaymentFailed, PaymentExpired, PaymentCancelled, PaymentRefunded:
		return true
	default:
		return false
	}
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
