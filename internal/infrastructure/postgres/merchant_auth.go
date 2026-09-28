package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

type MerchantAuthenticator struct {
	pool *pgxpool.Pool
}

func NewMerchantAuthenticator(pool *pgxpool.Pool) *MerchantAuthenticator {
	return &MerchantAuthenticator{pool: pool}
}

func (a *MerchantAuthenticator) Authenticate(ctx context.Context, plainTextKey string) (int64, error) {
	hash, err := application.HashMerchantAPIKey(plainTextKey)
	if err != nil {
		return 0, application.ErrInvalidMerchant
	}
	var merchantID int64
	err = a.pool.QueryRow(ctx, "SELECT id FROM merchants WHERE api_key_hash = $1 AND active = TRUE", hash).Scan(&merchantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, application.ErrInvalidMerchant
	}
	if err != nil {
		return 0, fmt.Errorf("authenticate merchant: %w", err)
	}
	return merchantID, nil
}
