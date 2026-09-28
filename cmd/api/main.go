package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/mockprovider"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
	httpapi "github.com/luqmanulhakimdev/payment-gateway/internal/interfaces/http"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	pool, err := postgres.NewPool(startupCtx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.ApplyMigrations(startupCtx, pool); err != nil {
		return err
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	createIntent := application.NewCreateIntent(postgres.NewIntentStore(pool))
	getPaymentStatus := application.NewGetPaymentStatus(postgres.NewPaymentStatusStore(pool))
	provider := mockprovider.New()
	createAttempt := application.NewCreatePaymentAttempt(postgres.NewAttemptStore(pool), provider)
	createRefund := application.NewCreateRefund(postgres.NewRefundStore(pool), provider)
	webhookKey, err := base64.StdEncoding.DecodeString(os.Getenv("WEBHOOK_ENCRYPTION_KEY"))
	if err != nil || len(webhookKey) != 32 {
		return errors.New("WEBHOOK_ENCRYPTION_KEY must be base64 for exactly 32 random bytes")
	}
	rateLimit, err := configuredRateLimit()
	if err != nil {
		return err
	}
	rateLimiter, err := application.NewMerchantRateLimiter(postgres.NewRateLimitStore(pool), rateLimit.requests, rateLimit.window)
	if err != nil {
		return err
	}
	handleWebhook := application.NewHandleWebhook(postgres.NewWebhookStore(pool), webhookKey, rateLimiter)
	authenticator := postgres.NewMerchantAuthenticator(pool)
	server := &http.Server{Addr: addr, Handler: httpapi.NewRouter(pool.Ping, createIntent, authenticator, createRefund, createAttempt, handleWebhook, rateLimiter, getPaymentStatus), ReadHeaderTimeout: 5 * time.Second}
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	log.Printf("http server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type rateLimitConfig struct {
	requests int
	window   time.Duration
}

func configuredRateLimit() (rateLimitConfig, error) {
	config := rateLimitConfig{requests: 60, window: time.Minute}
	if value := os.Getenv("RATE_LIMIT_REQUESTS"); value != "" {
		requests, err := strconv.Atoi(value)
		if err != nil || requests < 1 || requests > 100000 {
			return rateLimitConfig{}, errors.New("RATE_LIMIT_REQUESTS must be between 1 and 100000")
		}
		config.requests = requests
	}
	if value := os.Getenv("RATE_LIMIT_WINDOW"); value != "" {
		window, err := time.ParseDuration(value)
		if err != nil || window <= 0 {
			return rateLimitConfig{}, errors.New("RATE_LIMIT_WINDOW must be a positive Go duration")
		}
		config.window = window
	}
	return config, nil
}
