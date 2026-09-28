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

type RefundStore struct{ pool *pgxpool.Pool }

func NewRefundStore(pool *pgxpool.Pool) *RefundStore { return &RefundStore{pool: pool} }

func (s *RefundStore) WithinRefundTransaction(ctx context.Context, operation func(application.RefundTransaction) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refund transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	if err := operation(&refundTransaction{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refund transaction: %w", err)
	}
	return nil
}

type refundTransaction struct{ tx pgx.Tx }

func (tx *refundTransaction) PrepareRefund(ctx context.Context, merchantID int64, intentID, key, generatedID string, requestHash [32]byte, request application.RefundRequest) (application.Refund, bool, error) {
	var intent application.Refund
	var databaseID int64
	var intentAmount int64
	var intentStatus string
	err := tx.tx.QueryRow(ctx, `SELECT id,amount_minor,currency,status FROM payment_intents WHERE merchant_id=$1 AND public_id=$2 FOR UPDATE`, merchantID, intentID).Scan(&databaseID, &intentAmount, &intent.Currency, &intentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Refund{}, false, application.ErrRefundNotFound
	}
	if err != nil {
		return application.Refund{}, false, fmt.Errorf("load refundable intent: %w", err)
	}
	if intentStatus != string(domain.PaymentPaid) && intentStatus != string(domain.PaymentRefunded) {
		return application.Refund{}, false, domain.ErrInvalidTransition
	}
	var attemptID int64
	if err := tx.tx.QueryRow(ctx, `SELECT id,provider_reference FROM payment_attempts WHERE payment_intent_id=$1 AND status='PAID' AND provider_reference IS NOT NULL ORDER BY attempt_number DESC LIMIT 1`, databaseID).Scan(&attemptID, &intent.ProviderReference); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.Refund{}, false, application.ErrRefundNotFound
		}
		return application.Refund{}, false, fmt.Errorf("load captured payment attempt: %w", err)
	}
	var storedHash []byte
	err = tx.tx.QueryRow(ctx, `SELECT r.public_id,r.amount_minor,r.currency,r.status,r.reason,r.created_at,r.request_hash
		FROM refunds r WHERE r.payment_intent_id=$1 AND r.idempotency_key=$2`, databaseID, key).Scan(&intent.ID, &intent.AmountMinor, &intent.Currency, &intent.Status, &intent.Reason, &intent.CreatedAt, &storedHash)
	if err == nil {
		if len(storedHash) != len(requestHash) || !equalHash(storedHash, requestHash[:]) {
			return application.Refund{}, false, application.ErrRefundConflict
		}
		intent.PaymentIntentID = intentID
		intent.ProviderReference = ""
		if err := tx.tx.QueryRow(ctx, `SELECT pa.provider_reference FROM refunds r JOIN payment_attempts pa ON pa.id=r.payment_attempt_id WHERE r.payment_intent_id=$1 AND r.idempotency_key=$2`, databaseID, key).Scan(&intent.ProviderReference); err != nil {
			return application.Refund{}, false, fmt.Errorf("load retry provider reference: %w", err)
		}
		return intent, intent.Status != "PENDING", nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return application.Refund{}, false, fmt.Errorf("load idempotent refund: %w", err)
	}
	var reserved int64
	if err := tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(amount_minor),0) FROM refunds WHERE payment_intent_id=$1 AND status IN ('PENDING','SUCCEEDED')`, databaseID).Scan(&reserved); err != nil {
		return application.Refund{}, false, fmt.Errorf("sum existing refunds: %w", err)
	}
	if _, err := domain.ApplyRefund(domain.PaymentIntent{ID: intentID, MerchantID: merchantID, AmountMinor: intentAmount, Currency: intent.Currency, Status: domain.PaymentStatus(intentStatus)}, request.AmountMinor, reserved); err != nil {
		return application.Refund{}, false, err
	}
	if err := tx.tx.QueryRow(ctx, `INSERT INTO refunds(public_id,payment_intent_id,payment_attempt_id,amount_minor,currency,status,reason,idempotency_key,request_hash)
		VALUES($1,$2,$3,$4,$5,'PENDING',$6,$7,$8) RETURNING public_id,amount_minor,currency,status,reason,created_at`, generatedID, databaseID, attemptID, request.AmountMinor, intent.Currency, request.Reason, key, requestHash[:]).Scan(&intent.ID, &intent.AmountMinor, &intent.Currency, &intent.Status, &intent.Reason, &intent.CreatedAt); err != nil {
		return application.Refund{}, false, fmt.Errorf("reserve refund: %w", err)
	}
	intent.PaymentIntentID = intentID
	intent.ProviderReference = ""
	if err := tx.tx.QueryRow(ctx, "SELECT provider_reference FROM payment_attempts WHERE id=$1", attemptID).Scan(&intent.ProviderReference); err != nil {
		return application.Refund{}, false, fmt.Errorf("load provider reference: %w", err)
	}
	return intent, false, nil
}

func (tx *refundTransaction) FinalizeRefund(ctx context.Context, merchantID int64, intentID, refundID, providerReference string) (application.Refund, error) {
	var refund application.Refund
	var databaseID int64
	var status string
	err := tx.tx.QueryRow(ctx, `SELECT r.id,p.public_id,r.public_id,r.amount_minor,r.currency,r.status,r.reason,r.created_at
		FROM refunds r JOIN payment_intents p ON p.id=r.payment_intent_id
		WHERE p.merchant_id=$1 AND p.public_id=$2 AND r.public_id=$3 FOR UPDATE OF r`, merchantID, intentID, refundID).Scan(&databaseID, &refund.PaymentIntentID, &refund.ID, &refund.AmountMinor, &refund.Currency, &status, &refund.Reason, &refund.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Refund{}, application.ErrRefundNotFound
	}
	if err != nil {
		return application.Refund{}, fmt.Errorf("lock refund: %w", err)
	}
	if status == "SUCCEEDED" {
		refund.Status = status
		return refund, nil
	}
	if status != "PENDING" {
		return application.Refund{}, application.ErrRefundInProgress
	}
	if _, err := tx.tx.Exec(ctx, "UPDATE refunds SET status='SUCCEEDED',provider_reference=$2,updated_at=now() WHERE id=$1", databaseID, providerReference); err != nil {
		return application.Refund{}, fmt.Errorf("complete refund: %w", err)
	}
	var refunded, total int64
	if err := tx.tx.QueryRow(ctx, `SELECT COALESCE(sum(r.amount_minor),0),p.amount_minor FROM refunds r JOIN payment_intents p ON p.id=r.payment_intent_id WHERE r.payment_intent_id=(SELECT id FROM payment_intents WHERE public_id=$1) AND r.status='SUCCEEDED' GROUP BY p.amount_minor`, intentID).Scan(&refunded, &total); err != nil {
		return application.Refund{}, fmt.Errorf("sum successful refunds: %w", err)
	}
	if refunded == total {
		if _, err := tx.tx.Exec(ctx, "UPDATE payment_intents SET status='REFUNDED',updated_at=now() WHERE merchant_id=$1 AND public_id=$2 AND status='PAID'", merchantID, intentID); err != nil {
			return application.Refund{}, fmt.Errorf("mark intent refunded: %w", err)
		}
	}
	if _, err := tx.tx.Exec(ctx, `INSERT INTO audit_logs(merchant_id,actor_type,actor_id,action,entity_type,entity_id,details)
		VALUES($1,'merchant_api',NULL,'refund.succeeded','refund',$2,jsonb_build_object('payment_intent_id',$3,'amount_minor',$4,'currency',$5))`, merchantID, refundID, intentID, refund.AmountMinor, refund.Currency); err != nil {
		return application.Refund{}, fmt.Errorf("audit refund: %w", err)
	}
	refund.Status = "SUCCEEDED"
	return refund, nil
}

func equalHash(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for i := range left {
		diff |= left[i] ^ right[i]
	}
	return diff == 0
}

var _ application.RefundStore = (*RefundStore)(nil)
var _ application.RefundTransaction = (*refundTransaction)(nil)
