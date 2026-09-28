package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/mockprovider"
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
	olderThan := 5 * time.Minute
	if value := os.Getenv("RECONCILIATION_OLDER_THAN"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return errors.New("RECONCILIATION_OLDER_THAN must be a positive Go duration")
		}
		olderThan = parsed
	}
	batchSize := 100
	if value := os.Getenv("RECONCILIATION_BATCH_SIZE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1000 {
			return errors.New("RECONCILIATION_BATCH_SIZE must be between 1 and 1000")
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
	provider := mockprovider.New()
	reconciler := application.NewReconcilePayments(postgres.NewReconciliationStore(pool), provider)
	report, err := reconciler.Execute(ctx, olderThan, batchSize, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if report.Failed > 0 {
		return errors.New("one or more payment attempts could not be reconciled")
	}
	return nil
}
