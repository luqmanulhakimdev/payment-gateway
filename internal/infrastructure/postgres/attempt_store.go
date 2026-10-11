package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

type AttemptStore struct{ pool *pgxpool.Pool }

func NewAttemptStore(pool *pgxpool.Pool) *AttemptStore { return &AttemptStore{pool: pool} }

func (s *AttemptStore) WithinAttemptTransaction(ctx context.Context, operation func(application.AttemptTransaction) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin payment attempt transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	if err := operation(&attemptTransaction{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit payment attempt transaction: %w", err)
	}
	return nil
}

type attemptTransaction struct{ tx pgx.Tx }

func (tx *attemptTransaction) PrepareAttempt(ctx context.Context, merchantID int64, intentID, key, provider string) (application.PaymentAttempt, bool, int64, string, error) {
	var dbIntentID, amount int64
	var currency, status string
	err := tx.tx.QueryRow(ctx, `SELECT id,amount_minor,currency,status FROM payment_intents WHERE merchant_id=$1 AND public_id=$2 FOR UPDATE`, merchantID, intentID).Scan(&dbIntentID, &amount, &currency, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.PaymentAttempt{}, false, 0, "", application.ErrIntentNotFound
	}
	if err != nil {
		return application.PaymentAttempt{}, false, 0, "", fmt.Errorf("load payment intent: %w", err)
	}
	var attempt application.PaymentAttempt
	err = tx.tx.QueryRow(ctx, `SELECT id,attempt_number,status,provider,created_at FROM payment_attempts WHERE payment_intent_id=$1 AND idempotency_key=$2`, dbIntentID, key).Scan(&attempt.ID, &attempt.AttemptNumber, &attempt.Status, &attempt.Provider, &attempt.CreatedAt)
	if err == nil {
		attempt.PaymentIntentID = intentID
		return attempt, attempt.Status != "PENDING", amount, currency, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return application.PaymentAttempt{}, false, 0, "", fmt.Errorf("load idempotent attempt: %w", err)
	}
	if status != string(domain.PaymentCreated) {
		return application.PaymentAttempt{}, false, 0, "", application.ErrIntentNotPayable
	}
	err = tx.tx.QueryRow(ctx, `INSERT INTO payment_attempts(payment_intent_id,attempt_number,provider,status,idempotency_key)
		SELECT $1,COALESCE(max(attempt_number),0)+1,$2,'PENDING',$3 FROM payment_attempts WHERE payment_intent_id=$1
		RETURNING id,attempt_number,status,provider,created_at`, dbIntentID, provider, key).Scan(&attempt.ID, &attempt.AttemptNumber, &attempt.Status, &attempt.Provider, &attempt.CreatedAt)
	if err != nil {
		return application.PaymentAttempt{}, false, 0, "", fmt.Errorf("create payment attempt: %w", err)
	}
	if _, err := tx.tx.Exec(ctx, "UPDATE payment_intents SET status='PENDING',updated_at=now() WHERE id=$1", dbIntentID); err != nil {
		return application.PaymentAttempt{}, false, 0, "", fmt.Errorf("mark payment intent pending: %w", err)
	}
	attempt.PaymentIntentID = intentID
	return attempt, false, amount, currency, nil
}

func (tx *attemptTransaction) FinalizeAttempt(ctx context.Context, merchantID int64, intentID string, attemptID int64, providerReference string) (application.PaymentAttempt, error) {
	var attempt application.PaymentAttempt
	var intentDBID int64
	err := tx.tx.QueryRow(ctx, `SELECT a.id,a.attempt_number,a.status,a.provider,a.created_at,p.id
		FROM payment_attempts a JOIN payment_intents p ON p.id=a.payment_intent_id
		WHERE p.merchant_id=$1 AND p.public_id=$2 AND a.id=$3 AND a.status IN ('PENDING','AUTHORIZED') FOR UPDATE OF a`, merchantID, intentID, attemptID).Scan(&attempt.ID, &attempt.AttemptNumber, &attempt.Status, &attempt.Provider, &attempt.CreatedAt, &intentDBID)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.PaymentAttempt{}, application.ErrIntentNotPayable
	}
	if err != nil {
		return application.PaymentAttempt{}, fmt.Errorf("lock payment attempt: %w", err)
	}
	if attempt.Status == "AUTHORIZED" {
		attempt.PaymentIntentID = intentID
		return attempt, nil
	}
	if _, err := tx.tx.Exec(ctx, "UPDATE payment_attempts SET status='AUTHORIZED',provider_reference=$2,updated_at=now() WHERE id=$1", attempt.ID, providerReference); err != nil {
		return application.PaymentAttempt{}, fmt.Errorf("authorize attempt: %w", err)
	}
	if _, err := tx.tx.Exec(ctx, "UPDATE payment_intents SET status='AUTHORIZED',updated_at=now() WHERE id=$1 AND status='PENDING'", intentDBID); err != nil {
		return application.PaymentAttempt{}, fmt.Errorf("authorize intent: %w", err)
	}
	if _, err := tx.tx.Exec(ctx, `INSERT INTO audit_logs(merchant_id,actor_type,actor_id,action,entity_type,entity_id,details)
		VALUES($1,'system',NULL,'payment.authorized','payment_intent',$2,jsonb_build_object('attempt_number',$3::int,'provider',$4::text))`, merchantID, intentID, attempt.AttemptNumber, attempt.Provider); err != nil {
		return application.PaymentAttempt{}, fmt.Errorf("audit payment authorization: %w", err)
	}
	attempt.Status = "AUTHORIZED"
	attempt.PaymentIntentID = intentID
	return attempt, nil
}

var _ application.AttemptStore = (*AttemptStore)(nil)
var _ application.AttemptTransaction = (*attemptTransaction)(nil)
