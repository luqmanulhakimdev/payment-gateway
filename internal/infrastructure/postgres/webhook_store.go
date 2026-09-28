package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

type WebhookStore struct{ pool *pgxpool.Pool }

func NewWebhookStore(pool *pgxpool.Pool) *WebhookStore { return &WebhookStore{pool: pool} }

func (s *WebhookStore) GetWebhookSecret(ctx context.Context, merchantID int64) (string, error) {
	var secret string
	err := s.pool.QueryRow(ctx, "SELECT webhook_secret_encrypted FROM merchants WHERE id=$1 AND active=TRUE AND webhook_secret_encrypted IS NOT NULL", merchantID).Scan(&secret)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", application.ErrInvalidWebhookSignature
	}
	if err != nil {
		return "", fmt.Errorf("load merchant webhook secret: %w", err)
	}
	return secret, nil
}

func (s *WebhookStore) ProcessWebhook(ctx context.Context, merchantID int64, provider string, event application.WebhookEvent, payload []byte) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin webhook transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	var eventID int64
	err = tx.QueryRow(ctx, `INSERT INTO webhook_events(merchant_id,provider,provider_event_id,event_type,payload,signature_valid,status)
		VALUES($1,$2,$3,$4,$5::jsonb,TRUE,'RECEIVED') ON CONFLICT(merchant_id,provider,provider_event_id) DO NOTHING RETURNING id`, merchantID, provider, event.ID, event.Type, string(payload)).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		var status, existingType string
		var existingPayload []byte
		if err := tx.QueryRow(ctx, `SELECT id,status,event_type,payload FROM webhook_events WHERE merchant_id=$1 AND provider=$2 AND provider_event_id=$3 FOR UPDATE`, merchantID, provider, event.ID).Scan(&eventID, &status, &existingType, &existingPayload); err != nil {
			return false, fmt.Errorf("lock duplicate webhook: %w", err)
		}
		var existing application.WebhookEvent
		if err := json.Unmarshal(existingPayload, &existing); err != nil || existingType != event.Type || existing != event {
			return false, application.ErrWebhookConflict
		}
		if status == "PROCESSED" {
			return true, tx.Commit(ctx)
		}
		if _, err := tx.Exec(ctx, `UPDATE webhook_events SET status='PROCESSING',attempts=attempts+1,last_error=NULL,next_attempt_at=NULL WHERE id=$1`, eventID); err != nil {
			return false, fmt.Errorf("retry webhook event: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("persist webhook event: %w", err)
	} else {
		if _, err := tx.Exec(ctx, `UPDATE webhook_events SET status='PROCESSING',attempts=attempts+1 WHERE id=$1`, eventID); err != nil {
			return false, fmt.Errorf("start webhook processing: %w", err)
		}
	}
	var intentID int64
	var publicID, intentStatus, attemptStatus string
	var attemptID int64
	err = tx.QueryRow(ctx, `SELECT p.id,p.public_id,p.status,a.id,a.status
		FROM payment_attempts a JOIN payment_intents p ON p.id=a.payment_intent_id
		WHERE p.merchant_id=$1 AND a.provider=$2 AND a.provider_reference=$3
		FOR UPDATE OF p,a`, merchantID, provider, event.PaymentReference).Scan(&intentID, &publicID, &intentStatus, &attemptID, &attemptStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("webhook payment reference not found")
	}
	if err != nil {
		return false, fmt.Errorf("load webhook payment: %w", err)
	}
	var next string
	if event.Type == "payment.paid" {
		if intentStatus != "AUTHORIZED" || attemptStatus != "AUTHORIZED" {
			return false, fmt.Errorf("payment is not awaiting capture")
		}
		next = "PAID"
	} else {
		if (intentStatus != "PENDING" && intentStatus != "AUTHORIZED") || (attemptStatus != "PENDING" && attemptStatus != "AUTHORIZED") {
			return false, fmt.Errorf("payment cannot be failed from current state")
		}
		next = "FAILED"
	}
	if _, err := tx.Exec(ctx, `UPDATE payment_attempts SET status=$2,updated_at=now() WHERE id=$1`, attemptID, next); err != nil {
		return false, fmt.Errorf("update payment attempt: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE payment_intents SET status=$2,updated_at=now() WHERE id=$1`, intentID, next); err != nil {
		return false, fmt.Errorf("update payment intent: %w", err)
	}
	action := "payment." + stringsLower(next)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(merchant_id,actor_type,action,entity_type,entity_id,details)
		VALUES($1,'system',$2,'payment_intent',$3,jsonb_build_object('provider',$4::text,'provider_event_id',$5::text))`, merchantID, action, publicID, provider, event.ID); err != nil {
		return false, fmt.Errorf("audit webhook payment: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE webhook_events SET status='PROCESSED',processed_at=now(),last_error=NULL WHERE id=$1`, eventID); err != nil {
		return false, fmt.Errorf("complete webhook event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit webhook transaction: %w", err)
	}
	return false, nil
}

func stringsLower(value string) string {
	if value == "PAID" {
		return "paid"
	}
	return "failed"
}

var _ application.WebhookStore = (*WebhookStore)(nil)
