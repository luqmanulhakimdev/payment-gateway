package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type refundRecoveryStoreStub struct {
	items     []PendingRefund
	completed int
	before    time.Time
	limit     int
	err       error
}

func (s *refundRecoveryStoreStub) ListPendingRefunds(_ context.Context, before time.Time, limit int) ([]PendingRefund, error) {
	s.before, s.limit = before, limit
	return s.items, s.err
}
func (s *refundRecoveryStoreStub) CompletePendingRefund(_ context.Context, _ PendingRefund, ref string) error {
	if ref != "refund-ref" {
		return errors.New("wrong refund reference")
	}
	s.completed++
	return nil
}

func TestReconcileRefundsUsesStableProviderKeyAndCompletes(t *testing.T) {
	store := &refundRecoveryStoreStub{items: []PendingRefund{{MerchantID: 3, IntentID: "pi_1", RefundID: "rf_1", ProviderReference: "charge_ref", AmountMinor: 450, Currency: "IDR"}}}
	provider := &refundProviderStub{}
	service := NewReconcileRefunds(store, provider)
	now := time.Now().UTC()
	report, err := service.Execute(context.Background(), 5*time.Minute, 20, now)
	if err != nil || report.Checked != 1 || report.Recovered != 1 || report.Failed != 0 || store.completed != 1 || provider.request.IdempotencyKey != "refund-rf_1" || provider.request.ProviderReference != "charge_ref" || !store.before.Equal(now.Add(-5*time.Minute)) || store.limit != 20 {
		t.Fatalf("report=%#v err=%v store=%#v provider=%#v", report, err, store, provider)
	}
}

func TestReconcileRefundsLeavesProviderFailuresPending(t *testing.T) {
	store := &refundRecoveryStoreStub{items: []PendingRefund{{IntentID: "pi_1", RefundID: "rf_1", AmountMinor: 450, Currency: "IDR"}}}
	provider := &refundProviderStub{err: errors.New("timeout")}
	report, err := NewReconcileRefunds(store, provider).Execute(context.Background(), time.Minute, 10, time.Now())
	if err != nil || report.Checked != 1 || report.Failed != 1 || report.Recovered != 0 || store.completed != 0 {
		t.Fatalf("report=%#v err=%v store=%#v", report, err, store)
	}
}
