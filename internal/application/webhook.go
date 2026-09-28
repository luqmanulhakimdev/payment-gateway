package application

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var ErrWebhookEventInvalid = errors.New("invalid webhook event")
var ErrWebhookConflict = errors.New("webhook event ID conflicts with stored content")

type WebhookEvent struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	PaymentReference string `json:"payment_reference"`
}

type WebhookResult struct {
	Duplicate bool `json:"duplicate"`
}

type WebhookStore interface {
	GetWebhookSecret(context.Context, int64) (string, error)
	ProcessWebhook(context.Context, int64, string, WebhookEvent, []byte) (bool, error)
}

type HandleWebhook struct {
	store         WebhookStore
	encryptionKey []byte
	tolerance     time.Duration
}

func NewHandleWebhook(store WebhookStore, encryptionKey []byte) *HandleWebhook {
	return &HandleWebhook{store: store, encryptionKey: append([]byte(nil), encryptionKey...), tolerance: 5 * time.Minute}
}

func (h *HandleWebhook) Execute(ctx context.Context, merchantID int64, provider, signature string, body []byte, now time.Time) (WebhookResult, error) {
	if h == nil || h.store == nil || merchantID <= 0 || !validWebhookProvider(provider) || len(body) == 0 || len(body) > 1<<20 {
		return WebhookResult{}, ErrWebhookEventInvalid
	}
	encrypted, err := h.store.GetWebhookSecret(ctx, merchantID)
	if err != nil {
		if errors.Is(err, ErrInvalidWebhookSignature) {
			return WebhookResult{}, ErrInvalidWebhookSignature
		}
		return WebhookResult{}, fmt.Errorf("load webhook secret: %w", err)
	}
	secret, err := DecryptWebhookSecret(h.encryptionKey, encrypted)
	if err != nil {
		return WebhookResult{}, ErrInvalidWebhookSignature
	}
	if err := VerifyWebhookSignature(secret, body, signature, now, h.tolerance); err != nil {
		return WebhookResult{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var event WebhookEvent
	if err := decoder.Decode(&event); err != nil {
		return WebhookResult{}, ErrWebhookEventInvalid
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return WebhookResult{}, ErrWebhookEventInvalid
	}
	event.ID, event.Type, event.PaymentReference = strings.TrimSpace(event.ID), strings.TrimSpace(event.Type), strings.TrimSpace(event.PaymentReference)
	if event.ID == "" || len(event.ID) > 200 || event.PaymentReference == "" || len(event.PaymentReference) > 200 || (event.Type != "payment.paid" && event.Type != "payment.failed") {
		return WebhookResult{}, ErrWebhookEventInvalid
	}
	// Store only the allow-listed fields. Provider payloads never persist arbitrary data.
	safePayload, _ := json.Marshal(event)
	duplicate, err := h.store.ProcessWebhook(ctx, merchantID, provider, event, safePayload)
	if err != nil {
		return WebhookResult{}, err
	}
	return WebhookResult{Duplicate: duplicate}, nil
}

func validWebhookProvider(provider string) bool {
	if len(provider) == 0 || len(provider) > 50 || strings.TrimSpace(provider) != provider {
		return false
	}
	for _, ch := range provider {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

func EncryptWebhookSecret(key, secret []byte) (string, error) {
	if len(key) != 32 || len(secret) < 32 {
		return "", fmt.Errorf("encryption key must be 32 bytes and webhook secret at least 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, secret, []byte("payment-gateway-webhook-secret-v1"))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func DecryptWebhookSecret(key []byte, encoded string) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("invalid webhook encryption key")
	}
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("invalid encrypted webhook secret")
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	return aead.Open(nil, nonce, ciphertext, []byte("payment-gateway-webhook-secret-v1"))
}
