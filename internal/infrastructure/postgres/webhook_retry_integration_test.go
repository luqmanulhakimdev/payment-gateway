package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
)

func TestWebhookStoreIntegrationPersistsAndRetriesFailedEvent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := postgres.ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var merchantID, intentID int64
	if err := pool.QueryRow(ctx, `INSERT INTO merchants(name,api_key_hash) VALUES($1,$2) RETURNING id`, "Webhook retry test", suffix).Scan(&merchantID); err != nil {
		t.Fatal(err)
	}
	publicID := "pi_webhook_retry_" + suffix
	if err := pool.QueryRow(ctx, `INSERT INTO payment_intents(public_id,merchant_id,amount_minor,currency,status) VALUES($1,$2,700,'IDR','PENDING') RETURNING id`, publicID, merchantID).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	providerReference := "ref-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO payment_attempts(payment_intent_id,attempt_number,provider,provider_reference,status) VALUES($1,1,'mock',$2,'PENDING')`, intentID, providerReference); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE merchant_id=$1", merchantID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM webhook_events WHERE merchant_id=$1", merchantID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_attempts WHERE payment_intent_id=$1", intentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_intents WHERE id=$1", intentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM merchants WHERE id=$1", merchantID)
	})
	store := postgres.NewWebhookStore(pool)
	event := application.WebhookEvent{ID: "evt-" + suffix, Type: "payment.paid", PaymentReference: providerReference}
	payload, _ := json.Marshal(event)
	if _, err := store.ProcessWebhook(ctx, merchantID, "mock", event, payload); err == nil {
		t.Fatal("expected invalid transition to fail")
	}
	var status string
	var attempts int
	if err := pool.QueryRow(ctx, "SELECT status,attempts FROM webhook_events WHERE merchant_id=$1 AND provider_event_id=$2", merchantID, event.ID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "FAILED" || attempts != 1 {
		t.Fatalf("persisted failure status=%s attempts=%d", status, attempts)
	}
	if _, err := pool.Exec(ctx, `UPDATE payment_intents SET status='AUTHORIZED' WHERE id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE payment_attempts SET status='AUTHORIZED' WHERE payment_intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE webhook_events SET next_attempt_at=now()-interval '1 second' WHERE merchant_id=$1 AND provider_event_id=$2`, merchantID, event.ID); err != nil {
		t.Fatal(err)
	}
	processed, failed, err := store.RetryDueWebhookEvents(ctx, 10)
	if err != nil || processed != 1 || failed != 0 {
		t.Fatalf("retry processed=%d failed=%d err=%v", processed, failed, err)
	}
	if err := pool.QueryRow(ctx, "SELECT status FROM webhook_events WHERE merchant_id=$1 AND provider_event_id=$2", merchantID, event.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "PROCESSED" {
		t.Fatalf("event status=%s", status)
	}
	duplicate, err := store.ProcessWebhook(ctx, merchantID, "mock", event, payload)
	if err != nil || !duplicate {
		t.Fatalf("duplicate=%v err=%v", duplicate, err)
	}
}
