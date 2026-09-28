package application_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
)

func TestCreateIntentIntegrationIdempotency(t *testing.T) {
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
	var merchantID int64
	if err := pool.QueryRow(ctx, `INSERT INTO merchants (name, api_key_hash)
		VALUES ($1, $2) RETURNING id`, "Integration "+suffix, "test-hash-"+suffix).Scan(&merchantID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupIntentFixture(pool, merchantID) })

	createIntent := application.NewCreateIntent(postgres.NewIntentStore(pool))
	request := domain.CreatePaymentRequest{AmountMinor: 92500, Currency: "IDR", Description: "Order integration"}
	start := make(chan struct{})
	results := make(chan application.CreateIntentResult, 2)
	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, err := createIntent.Execute(ctx, merchantID, "same-request", request)
			results <- result
			errorsFound <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatalf("create payment intent: %v", err)
		}
	}

	var firstBody []byte
	var fresh, replayed int
	for result := range results {
		if result.StatusCode != 201 {
			t.Fatalf("status = %d, want 201", result.StatusCode)
		}
		if result.Replayed {
			replayed++
		} else {
			fresh++
		}
		if firstBody == nil {
			firstBody = result.Body
		} else if string(result.Body) != string(firstBody) {
			t.Fatalf("idempotent responses differ: %s vs %s", result.Body, firstBody)
		}
	}
	if fresh != 1 || replayed != 1 {
		t.Fatalf("fresh=%d replayed=%d, want one each", fresh, replayed)
	}

	if _, err := createIntent.Execute(ctx, merchantID, "same-request", domain.CreatePaymentRequest{AmountMinor: 92501, Currency: "IDR"}); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("same key with changed request error = %v, want conflict", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM payment_intents WHERE merchant_id = $1", merchantID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("payment intents = %d, want 1", count)
	}
}

func cleanupIntentFixture(pool *pgxpool.Pool, merchantID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, "DELETE FROM idempotency_keys WHERE merchant_id = $1", merchantID)
	_, _ = tx.Exec(ctx, "DELETE FROM payment_intents WHERE merchant_id = $1", merchantID)
	_, _ = tx.Exec(ctx, "DELETE FROM merchants WHERE id = $1", merchantID)
	_ = tx.Commit(ctx)
}
