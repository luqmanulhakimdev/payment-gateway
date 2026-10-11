package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type rateLimitStoreStub struct {
	key     string
	limit   int
	window  time.Duration
	allowed bool
	retry   time.Duration
	err     error
}

func (s *rateLimitStoreStub) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	s.key, s.limit, s.window = key, limit, window
	return s.allowed, s.retry, s.err
}

func TestMerchantRateLimiterScopesByMerchantAndOperation(t *testing.T) {
	store := &rateLimitStoreStub{allowed: false, retry: 17 * time.Second}
	limiter, err := NewMerchantRateLimiter(store, 12, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	allowed, retry, err := limiter.Check(context.Background(), 42, "refunds")
	if err != nil || allowed || retry != 17*time.Second || store.key != "merchant:42:refunds" || store.limit != 12 || store.window != time.Minute {
		t.Fatalf("check=(%v,%s,%v) store=%#v", allowed, retry, err, store)
	}
}

func TestMerchantRateLimiterRejectsInvalidConfigurationAndPropagatesStoreErrors(t *testing.T) {
	if _, err := NewMerchantRateLimiter(&rateLimitStoreStub{}, 0, time.Minute); !errors.Is(err, ErrInvalidRateLimit) {
		t.Fatalf("invalid configuration error=%v", err)
	}
	store := &rateLimitStoreStub{err: errors.New("database unavailable")}
	limiter, err := NewMerchantRateLimiter(store, 5, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := limiter.Check(context.Background(), 1, "payments"); err == nil {
		t.Fatal("expected store error")
	}
}
