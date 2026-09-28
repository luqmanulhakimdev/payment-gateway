package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
)

type merchantAuthFunc func(context.Context, string) (int64, error)

func (f merchantAuthFunc) Authenticate(ctx context.Context, key string) (int64, error) {
	return f(ctx, key)
}

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewRouter(nil, nil, nil, nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok\n" {
		t.Fatalf("unexpected health response: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestReadiness(t *testing.T) {
	for _, tt := range []struct {
		name  string
		check func(context.Context) error
		want  int
	}{
		{"ready", func(context.Context) error { return nil }, http.StatusOK},
		{"database unavailable", func(context.Context) error { return errors.New("unavailable") }, http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			NewRouter(tt.check, nil, nil, nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}

func TestCreateIntentRejectsUnknownCardFields(t *testing.T) {
	router := NewRouter(nil, application.NewCreateIntent(nil), merchantAuthFunc(func(context.Context, string) (int64, error) { return 1, nil }), nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/payment-intents", strings.NewReader(`{"amount_minor":1000,"currency":"IDR","card_number":"4111111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer key")
	request.Header.Set("Idempotency-Key", "test-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestCreateIntentRequiresMerchantAuthentication(t *testing.T) {
	router := NewRouter(nil, application.NewCreateIntent(nil), merchantAuthFunc(func(context.Context, string) (int64, error) {
		return 0, application.ErrInvalidMerchant
	}), nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/payment-intents", strings.NewReader(`{"amount_minor":1000,"currency":"IDR"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer invalid-key")
	request.Header.Set("Idempotency-Key", "test-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
