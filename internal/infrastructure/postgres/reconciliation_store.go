package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

type ReconciliationStore struct{ pool *pgxpool.Pool }

func NewReconciliationStore(pool *pgxpool.Pool) *ReconciliationStore {
	return &ReconciliationStore{pool: pool}
}

func (s *ReconciliationStore) ListStaleAttempts(ctx context.Context, provider string, before time.Time, limit int) ([]application.StaleAttempt, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id,p.public_id,a.provider,p.amount_minor,p.currency,a.created_at
		FROM payment_attempts a JOIN payment_intents p ON p.id=a.payment_intent_id
		WHERE a.status='PENDING' AND p.status='PENDING' AND a.provider=$1 AND a.created_at<$2
		ORDER BY a.created_at,a.id LIMIT $3`, provider, before, limit)
	if err != nil {
		return nil, fmt.Errorf("query stale payment attempts: %w", err)
	}
	defer rows.Close()
	var result []application.StaleAttempt
	for rows.Next() {
		var item application.StaleAttempt
		if err := rows.Scan(&item.ID, &item.PaymentIntentID, &item.Provider, &item.AmountMinor, &item.Currency, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan stale payment attempt: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale payment attempts: %w", err)
	}
	return result, nil
}

func (s *ReconciliationStore) ApplyReconciliation(ctx context.Context, attempt application.StaleAttempt, result application.ProviderResult) (bool, error) {
	next := string(result.Status)
	if next != "AUTHORIZED" && next != "PAID" && next != "FAILED" {
		return false, application.ErrInvalidReconciliation
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin reconciliation transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	var dbAttemptID, dbIntentID int64
	var publicID, attemptStatus, intentStatus string
	err = tx.QueryRow(ctx, `SELECT a.id,p.id,p.public_id,a.status,p.status
		FROM payment_attempts a JOIN payment_intents p ON p.id=a.payment_intent_id
		WHERE a.id=$1 AND p.public_id=$2 FOR UPDATE OF a,p`, attempt.ID, attempt.PaymentIntentID).Scan(&dbAttemptID, &dbIntentID, &publicID, &attemptStatus, &intentStatus)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock reconciliation payment: %w", err)
	}
	if attemptStatus != "PENDING" || intentStatus != "PENDING" {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE payment_attempts SET status=$2,provider_reference=$3,updated_at=now() WHERE id=$1`, dbAttemptID, next, result.Reference); err != nil {
		return false, fmt.Errorf("update reconciled attempt: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE payment_intents SET status=$2,updated_at=now() WHERE id=$1`, dbIntentID, next); err != nil {
		return false, fmt.Errorf("update reconciled intent: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(merchant_id,actor_type,action,entity_type,entity_id,details)
		SELECT p.merchant_id,'system','payment.reconciled','payment_intent',p.public_id,jsonb_build_object('attempt_id',$2::bigint,'provider',$3::text,'provider_status',$4::text)
		FROM payment_intents p WHERE p.id=$1`, dbIntentID, dbAttemptID, attempt.Provider, next); err != nil {
		return false, fmt.Errorf("audit reconciliation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit reconciliation: %w", err)
	}
	return true, nil
}

var _ application.ReconciliationStore = (*ReconciliationStore)(nil)
