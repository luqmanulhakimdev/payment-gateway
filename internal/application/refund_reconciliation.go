package application

import (
	"context"
	"fmt"
	"time"
)

type PendingRefund struct {
	MerchantID        int64
	IntentID          string
	RefundID          string
	ProviderReference string
	AmountMinor       int64
	Currency          string
}

type RefundRecoveryStore interface {
	ListPendingRefunds(context.Context, time.Time, int) ([]PendingRefund, error)
	CompletePendingRefund(context.Context, PendingRefund, string) error
}

type RefundRecoveryReport struct {
	Checked   int `json:"checked"`
	Recovered int `json:"recovered"`
	Failed    int `json:"failed"`
}

type ReconcileRefunds struct {
	store    RefundRecoveryStore
	provider RefundProvider
}

func NewReconcileRefunds(store RefundRecoveryStore, provider RefundProvider) *ReconcileRefunds {
	return &ReconcileRefunds{store: store, provider: provider}
}

func (r *ReconcileRefunds) Execute(ctx context.Context, olderThan time.Duration, limit int, now time.Time) (RefundRecoveryReport, error) {
	if r == nil || r.store == nil || r.provider == nil || olderThan <= 0 || limit < 1 || limit > 1000 {
		return RefundRecoveryReport{}, fmt.Errorf("invalid refund reconciliation configuration")
	}
	items, err := r.store.ListPendingRefunds(ctx, now.Add(-olderThan), limit)
	if err != nil {
		return RefundRecoveryReport{}, err
	}
	report := RefundRecoveryReport{Checked: len(items)}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		result, err := r.provider.Refund(ctx, RefundProviderRequest{
			PaymentIntentID: item.IntentID, ProviderReference: item.ProviderReference,
			AmountMinor: item.AmountMinor, Currency: item.Currency, IdempotencyKey: "refund-" + item.RefundID,
		})
		if err == nil && result.Reference != "" {
			err = r.store.CompletePendingRefund(ctx, item, result.Reference)
		}
		if err != nil || result.Reference == "" {
			report.Failed++
			continue
		}
		report.Recovered++
	}
	return report, nil
}
