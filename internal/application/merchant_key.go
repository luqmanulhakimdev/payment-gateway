package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidAPIKey = errors.New("invalid merchant API key")

func GenerateMerchantAPIKey() (plainText, hash string, err error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", "", err
	}
	plainText = base64.RawURLEncoding.EncodeToString(key[:])
	hash, err = HashMerchantAPIKey(plainText)
	return plainText, hash, err
}

func HashMerchantAPIKey(plainText string) (string, error) {
	if len(plainText) != 43 || strings.TrimSpace(plainText) != plainText {
		return "", ErrInvalidAPIKey
	}
	decoded, err := base64.RawURLEncoding.DecodeString(plainText)
	if err != nil || len(decoded) != 32 {
		return "", ErrInvalidAPIKey
	}
	digest := sha256.Sum256([]byte(plainText))
	return hex.EncodeToString(digest[:]), nil
}
