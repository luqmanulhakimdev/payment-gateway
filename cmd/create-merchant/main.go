package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/infrastructure/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	name := strings.TrimSpace(os.Getenv("MERCHANT_NAME"))
	if name == "" {
		return errors.New("MERCHANT_NAME is required")
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("WEBHOOK_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		return errors.New("WEBHOOK_ENCRYPTION_KEY must be base64 for exactly 32 random bytes")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.ApplyMigrations(ctx, pool); err != nil {
		return err
	}
	apiKey, apiHash, err := application.GenerateMerchantAPIKey()
	if err != nil {
		return err
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return err
	}
	webhookSecret := hex.EncodeToString(secret[:])
	encrypted, err := application.EncryptWebhookSecret(key, []byte(webhookSecret))
	if err != nil {
		return err
	}
	var merchantID int64
	if err := pool.QueryRow(ctx, `INSERT INTO merchants(name,api_key_hash,webhook_secret_encrypted) VALUES($1,$2,$3) RETURNING id`, name, apiHash, encrypted).Scan(&merchantID); err != nil {
		return fmt.Errorf("create merchant: %w", err)
	}
	// These credentials are shown once. Store them in a secret manager immediately.
	fmt.Printf("merchant_id=%d\nmerchant_name=%s\nmerchant_api_key=%s\nwebhook_signing_secret=%s\n", merchantID, name, apiKey, webhookSecret)
	return nil
}
