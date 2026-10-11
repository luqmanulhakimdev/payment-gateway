package application

import (
	"context"
	"errors"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

var ErrPaymentStatusNotFound = errors.New("payment status not found")

type PaymentStatusView struct {
	ID          string    `json:"id"`
	AmountMinor int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PaymentStatusStore interface {
	GetPaymentStatus(context.Context, int64, string) (PaymentStatusView, error)
}

type GetPaymentStatus struct{ store PaymentStatusStore }

func NewGetPaymentStatus(store PaymentStatusStore) *GetPaymentStatus {
	return &GetPaymentStatus{store: store}
}

func (q *GetPaymentStatus) Execute(ctx context.Context, merchantID int64, intentID string) (PaymentStatusView, error) {
	if q == nil || q.store == nil || merchantID <= 0 || intentID == "" {
		return PaymentStatusView{}, domain.ErrInvalidPayment
	}
	return q.store.GetPaymentStatus(ctx, merchantID, intentID)
}
