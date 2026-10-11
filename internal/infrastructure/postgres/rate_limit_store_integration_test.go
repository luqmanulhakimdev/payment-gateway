package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
)

func TestRateLimitStoreIntegrationFixedWindow(t *testing.T) {
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
	key := fmt.Sprintf("integration:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM rate_limit_counters WHERE scope_key=$1", key)
	})
	store := postgres.NewRateLimitStore(pool)
	for i := 0; i < 2; i++ {
		allowed, _, err := store.Allow(ctx, key, 2, time.Minute)
		if err != nil || !allowed {
			t.Fatalf("request %d allowed=%v err=%v", i+1, allowed, err)
		}
	}
	allowed, retryAfter, err := store.Allow(ctx, key, 2, time.Minute)
	if err != nil || allowed || retryAfter <= 0 {
		t.Fatalf("limited request allowed=%v retry=%s err=%v", allowed, retryAfter, err)
	}
}
