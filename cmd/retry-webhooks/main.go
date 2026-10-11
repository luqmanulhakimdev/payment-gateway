package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	batchSize := 100
	if value := os.Getenv("WEBHOOK_RETRY_BATCH_SIZE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1000 {
			return errors.New("WEBHOOK_RETRY_BATCH_SIZE must be between 1 and 1000")
		}
		batchSize = parsed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	processed, failed, err := postgres.NewWebhookStore(pool).RetryDueWebhookEvents(ctx, batchSize)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]int{"processed": processed, "failed": failed}); err != nil {
		return err
	}
	if failed > 0 {
		return errors.New("one or more webhook events could not be processed")
	}
	return nil
}
