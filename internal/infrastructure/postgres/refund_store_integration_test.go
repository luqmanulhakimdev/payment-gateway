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

func TestRefundStoreIntegrationPartialAndFullRefund(t *testing.T) {
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
	var merchantID, intentID, attemptID int64
	if err := pool.QueryRow(ctx, `INSERT INTO merchants(name,api_key_hash) VALUES($1,$2) RETURNING id`, "Refund integration", suffix).Scan(&merchantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO payment_intents(public_id,merchant_id,amount_minor,currency,status) VALUES($1,$2,1000,'IDR','PAID') RETURNING id`, "pi_refund_"+suffix, merchantID).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO payment_attempts(payment_intent_id,attempt_number,provider,provider_reference,status) VALUES($1,1,'mock',$2,'PAID') RETURNING id`, intentID, "mock_charge_"+suffix).Scan(&attemptID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE merchant_id=$1", merchantID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM refunds WHERE payment_intent_id=$1", intentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_attempts WHERE id=$1", attemptID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM payment_intents WHERE id=$1", intentID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM merchants WHERE id=$1", merchantID)
	})

	service := application.NewCreateRefund(postgres.NewRefundStore(pool), mockprovider.New())
	first, replayed, err := service.Execute(ctx, merchantID, "pi_refund_"+suffix, "partial-1", application.RefundRequest{AmountMinor: 600, Reason: "partial return"})
	if err != nil || replayed || first.Status != "SUCCEEDED" {
		t.Fatalf("first refund=(%#v,%t,%v)", first, replayed, err)
	}
	firstAgain, replayed, err := service.Execute(ctx, merchantID, "pi_refund_"+suffix, "partial-1", application.RefundRequest{AmountMinor: 600, Reason: "partial return"})
	if err != nil || !replayed || firstAgain.ID != first.ID {
		t.Fatalf("replayed refund=(%#v,%t,%v)", firstAgain, replayed, err)
	}
	second, _, err := service.Execute(ctx, merchantID, "pi_refund_"+suffix, "partial-2", application.RefundRequest{AmountMinor: 400})
	if err != nil || second.Status != "SUCCEEDED" {
		t.Fatalf("second refund=(%#v,%v)", second, err)
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM payment_intents WHERE id=$1", intentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "REFUNDED" {
		t.Fatalf("intent status=%s, want REFUNDED", status)
	}
}
