package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/luqmanulhakimdev/payment-gateway/internal/application"
	"github.com/luqmanulhakimdev/payment-gateway/internal/domain"
)

type MerchantAuthenticator interface {
	Authenticate(context.Context, string) (int64, error)
}

func NewRouter(checkDatabase func(context.Context) error, createIntent *application.CreateIntent, authenticator MerchantAuthenticator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if checkDatabase == nil || checkDatabase(ctx) != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
	mux.Handle("POST /v1/payment-intents", createIntentHandler{useCase: createIntent, authenticator: authenticator})
	return mux
}

type createIntentHandler struct {
	useCase       *application.CreateIntent
	authenticator MerchantAuthenticator
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (h createIntentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.authenticator == nil || h.useCase == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "payment service is not configured")
		return
	}
	merchantID, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	if r.Header.Get("Idempotency-Key") == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request domain.CreatePaymentRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is invalid")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.useCase.Execute(ctx, merchantID, r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidPayment), errors.Is(err, domain.ErrInvalidIdempotencyKey):
			writeError(w, http.StatusBadRequest, "invalid_request", "Payment request is invalid")
		case errors.Is(err, domain.ErrIdempotencyConflict):
			writeError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was already used for a different request")
		case errors.Is(err, application.ErrCustomerNotFound):
			writeError(w, http.StatusNotFound, "customer_not_found", "Customer does not exist for this merchant")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "Payment intent could not be created")
		}
		return
	}
	if result.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.StatusCode)
	_, _ = w.Write(result.Body)
}

func (h createIntentHandler) authenticate(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := r.Header.Get("Authorization")
	scheme, key, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || key == "" || strings.Contains(key, " ") {
		writeError(w, http.StatusUnauthorized, "unauthorized", "A valid merchant API key is required")
		return 0, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	merchantID, err := h.authenticator.Authenticate(ctx, key)
	if err != nil || merchantID <= 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "A valid merchant API key is required")
		return 0, false
	}
	return merchantID, true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var response errorResponse
	response.Error.Code = code
	response.Error.Message = message
	_ = json.NewEncoder(w).Encode(response)
}
