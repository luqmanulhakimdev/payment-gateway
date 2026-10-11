package application

import "testing"

func TestGenerateMerchantAPIKey(t *testing.T) {
	plainText, hash, err := GenerateMerchantAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(plainText) != 43 || len(hash) != 64 {
		t.Fatalf("key/hash lengths = %d/%d", len(plainText), len(hash))
	}
	verified, err := HashMerchantAPIKey(plainText)
	if err != nil || verified != hash {
		t.Fatalf("key hash did not round-trip: %v", err)
	}
}
