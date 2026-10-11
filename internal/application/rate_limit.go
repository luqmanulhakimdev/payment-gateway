package application

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidRateLimit = errors.New("invalid rate limit configuration")
var ErrRateLimitStoreUnavailable = errors.New("rate limit store unavailable")
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type RateLimitExceededError struct{ RetryAfter time.Duration }

func (e RateLimitExceededError) Error() string { return ErrRateLimitExceeded.Error() }
func (e RateLimitExceededError) Unwrap() error { return ErrRateLimitExceeded }

type RateLimitStore interface {
	Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error)
}

type MerchantRateLimiter struct {
	store  RateLimitStore
	limit  int
	window time.Duration
}

func NewMerchantRateLimiter(store RateLimitStore, limit int, window time.Duration) (*MerchantRateLimiter, error) {
	if store == nil || limit < 1 || window <= 0 {
		return nil, ErrInvalidRateLimit
	}
	return &MerchantRateLimiter{store: store, limit: limit, window: window}, nil
}

func (l *MerchantRateLimiter) Check(ctx context.Context, merchantID int64, scope string) (bool, time.Duration, error) {
	if merchantID <= 0 || scope == "" {
		return false, 0, ErrInvalidRateLimit
	}
	key := fmt.Sprintf("merchant:%d:%s", merchantID, scope)
	return l.store.Allow(ctx, key, l.limit, l.window)
}
