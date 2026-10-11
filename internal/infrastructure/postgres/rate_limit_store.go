package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

type RateLimitStore struct{ pool *pgxpool.Pool }

func NewRateLimitStore(pool *pgxpool.Pool) *RateLimitStore { return &RateLimitStore{pool: pool} }

func (s *RateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	seconds := window.Seconds()
	var allowed bool
	var retrySeconds float64
	err := s.pool.QueryRow(ctx, `INSERT INTO rate_limit_counters(scope_key,window_started_at,request_count)
		VALUES($1,statement_timestamp(),1)
		ON CONFLICT(scope_key) DO UPDATE SET
		window_started_at=CASE WHEN rate_limit_counters.window_started_at <= statement_timestamp()-($3*interval '1 second') THEN statement_timestamp() ELSE rate_limit_counters.window_started_at END,
		request_count=CASE WHEN rate_limit_counters.window_started_at <= statement_timestamp()-($3*interval '1 second') THEN 1 WHEN rate_limit_counters.request_count<=$2 THEN rate_limit_counters.request_count+1 ELSE rate_limit_counters.request_count END
		RETURNING request_count <= $2, GREATEST(0,EXTRACT(EPOCH FROM (window_started_at+($3*interval '1 second')-statement_timestamp())))`, key, limit, seconds).Scan(&allowed, &retrySeconds)
	if err == pgx.ErrNoRows {
		return false, 0, application.ErrInvalidRateLimit
	}
	if err != nil {
		return false, 0, fmt.Errorf("update rate limit counter: %w", err)
	}
	return allowed, time.Duration(retrySeconds * float64(time.Second)), nil
}

var _ application.RateLimitStore = (*RateLimitStore)(nil)
