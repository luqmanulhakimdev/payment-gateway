package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

type IntentStore struct {
	pool *pgxpool.Pool
}

func NewIntentStore(pool *pgxpool.Pool) *IntentStore {
	return &IntentStore{pool: pool}
}

func (s *IntentStore) WithinTransaction(ctx context.Context, operation func(application.IntentTransaction) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin payment intent transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	if err := operation(&intentTransaction{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit payment intent: %w", err)
	}
	return nil
}

type intentTransaction struct {
	tx pgx.Tx
}

func (tx *intentTransaction) ReserveIdempotency(ctx context.Context, merchantID int64, key string, requestHash [32]byte, expiresAt time.Time) (application.IdempotencyReservation, error) {
	if _, err := tx.tx.Exec(ctx, `DELETE FROM idempotency_keys
		WHERE merchant_id = $1 AND idempotency_key = $2 AND expires_at <= now()`, merchantID, key); err != nil {
		return application.IdempotencyReservation{}, fmt.Errorf("remove expired idempotency key: %w", err)
	}
	var id int64
	err := tx.tx.QueryRow(ctx, `INSERT INTO idempotency_keys
		(merchant_id, idempotency_key, request_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (merchant_id, idempotency_key) DO NOTHING
		RETURNING id`, merchantID, key, requestHash[:], expiresAt).Scan(&id)
	if err == nil {
		return application.IdempotencyReservation{}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return application.IdempotencyReservation{}, fmt.Errorf("reserve idempotency key: %w", err)
	}

	var storedHash []byte
	var responseCode *int32
	var responseBody []byte
	err = tx.tx.QueryRow(ctx, `SELECT request_hash, response_status, response_body
		FROM idempotency_keys WHERE merchant_id = $1 AND idempotency_key = $2 FOR UPDATE`, merchantID, key).Scan(&storedHash, &responseCode, &responseBody)
	if err != nil {
		return application.IdempotencyReservation{}, fmt.Errorf("load idempotency key: %w", err)
	}
	var stored [32]byte
	if len(storedHash) != len(stored) {
		return application.IdempotencyReservation{}, domain.ErrIdempotencyConflict
	}
	copy(stored[:], storedHash)
	if err := domain.EnsureSameIdempotentRequest(stored, requestHash); err != nil {
		return application.IdempotencyReservation{}, err
	}
	if responseCode == nil || len(responseBody) == 0 {
		return application.IdempotencyReservation{}, domain.ErrIdempotencyInProgress
	}
	return application.IdempotencyReservation{Replayed: true, ResponseCode: int(*responseCode), ResponseBody: responseBody}, nil
}

func (tx *intentTransaction) CreateIntent(ctx context.Context, record application.IntentRecord) (application.CreatedIntent, error) {
	var customerID *int64
	if record.CustomerRef != "" {
		var id int64
		err := tx.tx.QueryRow(ctx, `SELECT id FROM customers WHERE merchant_id = $1 AND external_id = $2`, record.MerchantID, record.CustomerRef).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.CreatedIntent{}, application.ErrCustomerNotFound
		}
		if err != nil {
			return application.CreatedIntent{}, fmt.Errorf("find merchant customer: %w", err)
		}
		customerID = &id
	}
	metadata, err := json.Marshal(record.Metadata)
	if record.Metadata == nil {
		metadata = []byte("{}")
	}
	if err != nil {
		return application.CreatedIntent{}, fmt.Errorf("encode payment metadata: %w", err)
	}
	var intent application.CreatedIntent
	err = tx.tx.QueryRow(ctx, `INSERT INTO payment_intents
		(public_id, merchant_id, customer_id, amount_minor, currency, status, description, metadata)
		VALUES ($1, $2, $3, $4, $5, 'CREATED', $6, $7::jsonb)
		RETURNING id, public_id, status, amount_minor, currency, created_at`,
		record.PublicID, record.MerchantID, customerID, record.AmountMinor, record.Currency, record.Description, string(metadata),
	).Scan(&intent.DatabaseID, &intent.ID, &intent.Status, &intent.AmountMinor, &intent.Currency, &intent.CreatedAt)
	if err != nil {
		return application.CreatedIntent{}, fmt.Errorf("insert payment intent: %w", err)
	}
	return intent, nil
}

func (tx *intentTransaction) CompleteIdempotency(ctx context.Context, merchantID int64, key string, statusCode int, body []byte, paymentIntentID int64) error {
	tag, err := tx.tx.Exec(ctx, `UPDATE idempotency_keys SET response_status = $3, response_body = $4::jsonb, payment_intent_id = $5
		WHERE merchant_id = $1 AND idempotency_key = $2`, merchantID, key, statusCode, string(body), paymentIntentID)
	if err != nil {
		return fmt.Errorf("complete idempotency response: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrIdempotencyInProgress
	}
	return nil
}

var _ application.IntentStore = (*IntentStore)(nil)
var _ application.IntentTransaction = (*intentTransaction)(nil)
