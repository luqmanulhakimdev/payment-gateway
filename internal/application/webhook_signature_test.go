package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"
)

func sign(secret []byte, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(mac, "%s.", timestamp)
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	secret := []byte("0123456789abcdef0123456789abcdef")
	body := []byte(`{"id":"evt_123"}`)
	timestamp := fmt.Sprintf("%d", now.Unix())
	header := "t=" + timestamp + ",v1=" + sign(secret, timestamp, body)
	if err := VerifyWebhookSignature(secret, body, header, now, 5*time.Minute); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifyWebhookSignature(secret, []byte(`{"id":"evt_other"}`), header, now, 5*time.Minute); !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("modified payload error = %v", err)
	}
}

func TestVerifyWebhookSignatureRejectsStaleAndWeakInputs(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	secret := []byte("0123456789abcdef0123456789abcdef")
	body := []byte("event")
	oldTimestamp := fmt.Sprintf("%d", now.Add(-10*time.Minute).Unix())
	header := "t=" + oldTimestamp + ",v1=" + sign(secret, oldTimestamp, body)
	if err := VerifyWebhookSignature(secret, body, header, now, 5*time.Minute); !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("stale signature error = %v", err)
	}
	if err := VerifyWebhookSignature([]byte("short"), body, header, now, 5*time.Minute); !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("weak secret error = %v", err)
	}
}
