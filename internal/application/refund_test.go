package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type refundStoreStub struct {
	refund      Refund
	key         string
	requestHash [32]byte
	replay      bool
	finalized   int
}
type refundTxStub struct{ store *refundStoreStub }

func (s *refundStoreStub) WithinRefundTransaction(_ context.Context, fn func(RefundTransaction) error) error {
	return fn(refundTxStub{s})
}
func (tx refundTxStub) PrepareRefund(_ context.Context, _ int64, intentID, key, generated string, hash [32]byte, req RefundRequest) (Refund, bool, error) {
	s := tx.store
	s.key = key
	s.requestHash = hash
	if s.refund.ID == "" {
		s.refund = Refund{ID: generated, PaymentIntentID: intentID, AmountMinor: req.AmountMinor, Currency: "IDR", Status: "PENDING", CreatedAt: time.Now(), ProviderReference: "charge-ref"}
	}
	return s.refund, s.replay, nil
}
func (tx refundTxStub) FinalizeRefund(_ context.Context, _ int64, intentID, refundID, providerRef string) (Refund, error) {
	tx.store.finalized++
	tx.store.refund.Status = "SUCCEEDED"
	tx.store.refund.ProviderReference = providerRef
	return tx.store.refund, nil
}

type refundProviderStub struct {
	request RefundProviderRequest
	calls   int
	err     error
}

func (p *refundProviderStub) Refund(_ context.Context, request RefundProviderRequest) (RefundProviderResult, error) {
	p.calls++
	p.request = request
	if p.err != nil {
		return RefundProviderResult{}, p.err
	}
	return RefundProviderResult{Reference: "refund-ref"}, nil
}

func TestCreateRefundUsesIdempotencyAndProviderReference(t *testing.T) {
	store := &refundStoreStub{}
	provider := &refundProviderStub{}
	service := NewCreateRefund(store, provider)
	got, replayed, err := service.Execute(context.Background(), 9, "pi_123", "refund-key-1", RefundRequest{AmountMinor: 450, Reason: "duplicate"})
	if err != nil {
		t.Fatal(err)
	}
	if replayed || got.Status != "SUCCEEDED" || got.AmountMinor != 450 || provider.request.ProviderReference != "charge-ref" || provider.request.IdempotencyKey != "refund-"+got.ID {
		t.Fatalf("refund=%#v replay=%t provider=%#v", got, replayed, provider.request)
	}
	if store.finalized != 1 {
		t.Fatalf("finalizations=%d, want 1", store.finalized)
	}
}

func TestCreateRefundRetriesPendingReservationWithSameProviderKey(t *testing.T) {
	store := &refundStoreStub{}
	provider := &refundProviderStub{err: errors.New("timeout")}
	service := NewCreateRefund(store, provider)
	request := RefundRequest{AmountMinor: 250}
	_, _, err := service.Execute(context.Background(), 1, "pi_retry", "refund-retry", request)
	if !errors.Is(err, ErrRefundProviderUnavailable) {
		t.Fatalf("first error=%v", err)
	}
	firstID, firstKey := store.refund.ID, provider.request.IdempotencyKey
	provider.err = nil
	got, _, err := service.Execute(context.Background(), 1, "pi_retry", "refund-retry", request)
	if err != nil || got.Status != "SUCCEEDED" || got.ID != firstID || provider.request.IdempotencyKey != firstKey {
		t.Fatalf("retry=(%#v,%v), firstID=%s firstKey=%s key=%s", got, err, firstID, firstKey, provider.request.IdempotencyKey)
	}
}

func TestCreateRefundReplaysCompletedRefundWithoutCallingProvider(t *testing.T) {
	store := &refundStoreStub{refund: Refund{ID: "rf_existing", PaymentIntentID: "pi_123", AmountMinor: 100, Currency: "IDR", Status: "SUCCEEDED"}, replay: true}
	provider := &refundProviderStub{}
	got, replayed, err := NewCreateRefund(store, provider).Execute(context.Background(), 9, "pi_123", "refund-key-2", RefundRequest{AmountMinor: 100})
	if err != nil || !replayed || got.ID != "rf_existing" || provider.calls != 0 {
		t.Fatalf("refund=%#v replay=%t err=%v provider_calls=%d", got, replayed, err, provider.calls)
	}
}
