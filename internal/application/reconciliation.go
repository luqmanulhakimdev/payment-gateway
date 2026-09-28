package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidReconciliation = errors.New("invalid reconciliation request")

type StaleAttempt struct {
	ID              int64
	PaymentIntentID string
	Provider        string
	AmountMinor     int64
	Currency        string
	CreatedAt       time.Time
}

type ReconciliationStore interface {
	ListStaleAttempts(context.Context, string, time.Time, int) ([]StaleAttempt, error)
	ApplyReconciliation(context.Context, StaleAttempt, ProviderResult) (bool, error)
}

type ReconciliationReport struct {
	Checked   int `json:"checked"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Failed    int `json:"failed"`
}

type ReconcilePayments struct {
	store    ReconciliationStore
	provider PaymentReconciler
}

func NewReconcilePayments(store ReconciliationStore, provider PaymentReconciler) *ReconcilePayments {
	return &ReconcilePayments{store: store, provider: provider}
}

func (r *ReconcilePayments) Execute(ctx context.Context, olderThan time.Duration, batchSize int, now time.Time) (ReconciliationReport, error) {
	if r == nil || r.store == nil || r.provider == nil || olderThan <= 0 || batchSize < 1 || batchSize > 1000 {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	providerName := r.provider.Name()
	if providerName == "" {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	attempts, err := r.store.ListStaleAttempts(ctx, providerName, now.Add(-olderThan), batchSize)
	if err != nil {
		return ReconciliationReport{}, fmt.Errorf("list stale payment attempts: %w", err)
	}
	report := ReconciliationReport{Checked: len(attempts)}
	for _, attempt := range attempts {
		request := ProviderRequest{PaymentIntentID: attempt.PaymentIntentID, AttemptID: attempt.ID, AmountMinor: attempt.AmountMinor, Currency: attempt.Currency, IdempotencyKey: fmt.Sprintf("payment-attempt-%d", attempt.ID)}
		result, lookupErr := r.provider.LookupPayment(ctx, request)
		if lookupErr != nil {
			report.Failed++
			continue
		}
		if result.Status != ProviderAuthorized && result.Status != ProviderPaid && result.Status != ProviderFailed {
			report.Unchanged++
			continue
		}
		if result.Reference == "" {
			report.Failed++
			continue
		}
		changed, applyErr := r.store.ApplyReconciliation(ctx, attempt, result)
		if applyErr != nil {
			report.Failed++
			continue
		}
		if changed {
			report.Updated++
		} else {
			report.Unchanged++
		}
	}
	return report, nil
}
