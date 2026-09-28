package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type attemptStoreStub struct {
	attempt   PaymentAttempt
	replay    bool
	prepared  int
	finalized int
}
type attemptTxStub struct{ store *attemptStoreStub }

func (s *attemptStoreStub) WithinAttemptTransaction(_ context.Context, fn func(AttemptTransaction) error) error {
	return fn(attemptTxStub{s})
}
func (tx attemptTxStub) PrepareAttempt(_ context.Context, _ int64, intentID, key, provider string) (PaymentAttempt, bool, int64, string, error) {
	tx.store.prepared++
	if tx.store.attempt.ID == 0 {
		tx.store.attempt = PaymentAttempt{ID: 11, PaymentIntentID: intentID, AttemptNumber: 1, Status: "PENDING", Provider: provider, CreatedAt: time.Now()}
	}
	return tx.store.attempt, tx.store.replay, 700, "IDR", nil
}
func (tx attemptTxStub) FinalizeAttempt(_ context.Context, _ int64, intentID string, attemptID int64, _ string) (PaymentAttempt, error) {
	tx.store.finalized++
	tx.store.attempt.Status = "AUTHORIZED"
	return tx.store.attempt, nil
}

type paymentProviderStub struct {
	calls   int
	request ProviderRequest
	err     error
}

func (*paymentProviderStub) Name() string { return "stub" }
func (p *paymentProviderStub) CreatePayment(_ context.Context, r ProviderRequest) (ProviderResult, error) {
	p.calls++
	p.request = r
	if p.err != nil {
		return ProviderResult{}, p.err
	}
	return ProviderResult{Reference: "charge-ref", Status: ProviderAuthorized}, nil
}

func TestCreatePaymentAttemptUsesStableProviderKey(t *testing.T) {
	store := &attemptStoreStub{}
	provider := &paymentProviderStub{}
	got, replay, err := NewCreatePaymentAttempt(store, provider).Execute(context.Background(), 4, "pi_1", "attempt-key")
	if err != nil || replay || got.Status != "AUTHORIZED" || provider.request.IdempotencyKey != "payment-attempt-11" || provider.request.AmountMinor != 700 {
		t.Fatalf("attempt=(%#v,%t,%v) provider=%#v", got, replay, err, provider.request)
	}
}
func TestCreatePaymentAttemptReplaysAuthorizationWithoutProvider(t *testing.T) {
	store := &attemptStoreStub{attempt: PaymentAttempt{ID: 11, PaymentIntentID: "pi_1", AttemptNumber: 1, Status: "AUTHORIZED"}, replay: true}
	provider := &paymentProviderStub{}
	got, replay, err := NewCreatePaymentAttempt(store, provider).Execute(context.Background(), 4, "pi_1", "attempt-key")
	if err != nil || !replay || got.Status != "AUTHORIZED" || provider.calls != 0 {
		t.Fatalf("attempt=(%#v,%t,%v) provider calls=%d", got, replay, err, provider.calls)
	}
}
func TestCreatePaymentAttemptLeavesPendingOnProviderTimeout(t *testing.T) {
	store := &attemptStoreStub{}
	provider := &paymentProviderStub{err: errors.New("timeout")}
	_, _, err := NewCreatePaymentAttempt(store, provider).Execute(context.Background(), 4, "pi_1", "attempt-key")
	if !errors.Is(err, ErrAttemptProviderUnavailable) || store.finalized != 0 {
		t.Fatalf("error=%v finalized=%d", err, store.finalized)
	}
}
