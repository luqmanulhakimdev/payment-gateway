package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/mockprovider"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
)

func TestAttemptStoreIntegrationIdempotentAuthorization(t *testing.T) {
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
	if err := pool.QueryRow(ctx, `INSERT INTO merchants(name,api_key_hash) VALUES($1,$2) RETURNING id`, "Attempt test", suffix).Scan(&merchantID); err != nil {
		t.Fatal(err)
	}
	publicID := "pi_attempt_" + suffix
	if err := pool.QueryRow(ctx, `INSERT INTO payment_intents(public_id,merchant_id,amount_minor,currency) VALUES($1,$2,850,'IDR') RETURNING id`, publicID, merchantID).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE merchant_id=$1", merchantID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_attempts WHERE payment_intent_id=$1", intentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_intents WHERE id=$1", intentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM merchants WHERE id=$1", merchantID)
	})
	service := application.NewCreatePaymentAttempt(postgres.NewAttemptStore(pool), mockprovider.New())
	first, replayed, err := service.Execute(ctx, merchantID, publicID, "attempt-1")
	if err != nil || replayed || first.Status != "AUTHORIZED" {
		t.Fatalf("first attempt=(%#v,%t,%v)", first, replayed, err)
	}
	second, replayed, err := service.Execute(ctx, merchantID, publicID, "attempt-1")
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay=(%#v,%t,%v)", second, replayed, err)
	}
	var status string
	var count int
	if err := pool.QueryRow(ctx, "SELECT status FROM payment_intents WHERE id=$1", intentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM payment_attempts WHERE payment_intent_id=$1", intentID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if status != "AUTHORIZED" || count != 1 {
		t.Fatalf("intent status=%s attempts=%d", status, count)
	}
}
