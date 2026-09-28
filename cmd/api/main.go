package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	createRefund := application.NewCreateRefund(postgres.NewRefundStore(pool), mockprovider.New())
	authenticator := postgres.NewMerchantAuthenticator(pool)
	server := &http.Server{Addr: addr, Handler: httpapi.NewRouter(pool.Ping, createIntent, authenticator, createRefund), ReadHeaderTimeout: 5 * time.Second}
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
