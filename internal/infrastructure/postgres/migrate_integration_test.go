package postgres

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMigrationsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}

	var tableCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name IN
		('merchants','customers','payment_intents','payment_methods','payment_attempts','idempotency_keys','webhook_events','refunds','audit_logs')`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 9 {
		t.Fatalf("core table count = %d, want 9", tableCount)
	}
}
