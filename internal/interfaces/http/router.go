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

func NewRouter(checkDatabase func(context.Context) error, createIntent *application.CreateIntent, authenticator MerchantAuthenticator, createRefund *application.CreateRefund, createAttempt *application.CreatePaymentAttempt) http.Handler {
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
	mux.Handle("POST /v1/payment-intents/{intentID}/refunds", createRefundHandler{useCase: createRefund, authenticator: authenticator})
	mux.Handle("POST /v1/payment-intents/{intentID}/attempts", createAttemptHandler{useCase: createAttempt, authenticator: authenticator})
	return mux
}

type createAttemptHandler struct {
	useCase       *application.CreatePaymentAttempt
	authenticator MerchantAuthenticator
}

func (h createAttemptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.useCase == nil || h.authenticator == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "payment attempt service is not configured")
		return
	}
	merchantID, ok := (createIntentHandler{authenticator: h.authenticator}).authenticate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) != 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Payment attempt requests must not include a body")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	attempt, replayed, err := h.useCase.Execute(ctx, merchantID, r.PathValue("intentID"), key)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidIdempotencyKey), errors.Is(err, domain.ErrInvalidPayment):
			writeError(w, http.StatusBadRequest, "invalid_request", "Payment attempt request is invalid")
		case errors.Is(err, application.ErrIntentNotFound):
			writeError(w, http.StatusNotFound, "payment_intent_not_found", "Payment intent does not exist")
		case errors.Is(err, application.ErrIntentNotPayable):
			writeError(w, http.StatusConflict, "payment_intent_not_payable", "Payment intent cannot accept another attempt")
		case errors.Is(err, application.ErrAttemptProviderUnavailable):
			writeError(w, http.StatusBadGateway, "provider_unavailable", "Payment provider could not complete the request")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "Payment attempt could not be created")
		}
		return
	}
	if replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(attempt)
}

type createRefundHandler struct {
	useCase       *application.CreateRefund
	authenticator MerchantAuthenticator
}

func (h createRefundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.useCase == nil || h.authenticator == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "refund service is not configured")
		return
	}
	merchantID, ok := (createIntentHandler{authenticator: h.authenticator}).authenticate(w, r)
	if !ok {
		return
	}
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key_required", "Idempotency-Key header is required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request application.RefundRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is invalid")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	refund, replayed, err := h.useCase.Execute(ctx, merchantID, r.PathValue("intentID"), key, request)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidPayment), errors.Is(err, domain.ErrInvalidIdempotencyKey):
			writeError(w, http.StatusBadRequest, "invalid_request", "Refund request is invalid")
		case errors.Is(err, domain.ErrRefundExceedsAmount):
			writeError(w, http.StatusConflict, "refund_exceeds_captured_amount", "Refund amount exceeds the remaining captured amount")
		case errors.Is(err, application.ErrRefundNotFound):
			writeError(w, http.StatusNotFound, "payment_not_found", "A refundable payment was not found")
		case errors.Is(err, application.ErrRefundConflict):
			writeError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was used for a different refund request")
		case errors.Is(err, application.ErrRefundInProgress):
			writeError(w, http.StatusConflict, "refund_in_progress", "Refund is being processed")
		case errors.Is(err, domain.ErrInvalidTransition):
			writeError(w, http.StatusConflict, "payment_not_refundable", "Only paid payment intents can be refunded")
		case errors.Is(err, application.ErrRefundProviderUnavailable):
			writeError(w, http.StatusBadGateway, "refund_provider_unavailable", "Refund provider could not complete the request")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "Refund could not be completed")
		}
		return
	}
	if replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(refund)
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
