package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidWebhookSignature = errors.New("invalid webhook signature")

// VerifyWebhookSignature validates a t=...,v1=... HMAC-SHA256 signature.
// The signed message is the timestamp, a period, and the exact raw request body.
func VerifyWebhookSignature(secret, body []byte, header string, now time.Time, tolerance time.Duration) error {
	if len(secret) < 32 || tolerance <= 0 {
		return ErrInvalidWebhookSignature
	}
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || value == "" {
			continue
		}
		switch key {
		case "t":
			if timestamp != "" {
				return ErrInvalidWebhookSignature
			}
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	unixSeconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 {
		return ErrInvalidWebhookSignature
	}
	signedAt := time.Unix(unixSeconds, 0)
	if delta := now.Sub(signedAt); delta > tolerance || delta < -tolerance {
		return ErrInvalidWebhookSignature
	}

	mac := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(mac, "%s.", timestamp)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		provided, decodeErr := hex.DecodeString(signature)
		if decodeErr == nil && hmac.Equal(expected, provided) {
			return nil
		}
	}
	return ErrInvalidWebhookSignature
}
