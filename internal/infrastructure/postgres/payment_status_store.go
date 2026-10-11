package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

type PaymentStatusStore struct{ pool *pgxpool.Pool }

func NewPaymentStatusStore(pool *pgxpool.Pool) *PaymentStatusStore {
	return &PaymentStatusStore{pool: pool}
}

func (s *PaymentStatusStore) GetPaymentStatus(ctx context.Context, merchantID int64, intentID string) (application.PaymentStatusView, error) {
	var status application.PaymentStatusView
	err := s.pool.QueryRow(ctx, `SELECT public_id,amount_minor,currency,status,created_at,updated_at
		FROM payment_intents WHERE merchant_id=$1 AND public_id=$2`, merchantID, intentID).Scan(&status.ID, &status.AmountMinor, &status.Currency, &status.Status, &status.CreatedAt, &status.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.PaymentStatusView{}, application.ErrPaymentStatusNotFound
	}
	if err != nil {
		return application.PaymentStatusView{}, fmt.Errorf("load payment status: %w", err)
	}
	return status, nil
}

var _ application.PaymentStatusStore = (*PaymentStatusStore)(nil)
