// The OTS Server API Component
//
// Objectives:
// - Provides HTTP API endpoints for encrypted zero-knowledge one-time secret creation and one-time retrieval.
// - Enforces rate limiting per client IP, individual payload size limits, and total instance storage caps.
// - Provides dynamic server customization parameters to frontend and CLI clients via /api/settings.
//
// Core Components:
// - APIServer: The core REST controller encapsulating storage backend, IP rate limiter, and Prometheus collector.
// - handleCreate: Validates payload boundaries, rate limits, storage caps, and persists encrypted blobs.
// - handleRead: One-time atomic secret fetch & burn logic.
// - handleSettings: Exposes customization parameters (e.g. accepted file extensions, expiry choices).
//
// Data Flow:
// 1. Client POST /api/create -> IP Rate Limiter check -> MaxBytesReader / Storage Cap check -> Store.Create() -> Returns secret_id.
// 2. Client GET /api/get/{id} -> Store.ReadAndDestroy() -> Returns encrypted blob & deletes secret permanently -> Decrements storage bytes.
package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/Luzifer/ots/pkg/metrics"
	"github.com/Luzifer/ots/pkg/storage"
)

const (
	errorReasonInvalidExpiry  = "invalid_expiry"
	errorReasonInvalidJSON    = "invalid_json"
	errorReasonSecretMissing  = "secret_missing"
	errorReasonSecretNotFound = "secret_not_found"
	errorReasonSecretSize     = "secret_size"
	errorReasonStorageError   = "storage_error"

	maxExpirySeconds = int64(1<<63-1) / int64(time.Second)
)

// APIServer encapsulates storage backends, rate limiting engines, atomic storage tracking, and Prometheus metrics.
type APIServer struct {
	collector    *metrics.Collector
	rateLimiter  *ipRateLimiter
	store        storage.Storage
	storageBytes atomic.Int64
}

type (
	apiServer = APIServer
)

// apiResponse standardizes JSON responses returned by the API.
type apiResponse struct {
	Success   bool       `json:"success"`
	Error     string     `json:"error,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Secret    string     `json:"secret,omitempty"` //#nosec:G117 // This application works with secrets
	SecretID  string     `json:"secret_id,omitempty"`
}

// apiRequest models incoming JSON secret submission payloads.
type apiRequest struct {
	Secret string `json:"secret"` //#nosec:G117 // This application works with secrets
}

// NewAPI initializes an APIServer instance with rate limiting, storage, and metric collection capabilities.
func NewAPI(s storage.Storage, c *metrics.Collector) *APIServer {
	return &APIServer{
		collector:   c,
		rateLimiter: newIPRateLimiter(cust.RateLimitCreate, time.Minute),
		store:       s,
	}
}

//nolint:revive // Legacy internal helper constructor
func newAPI(s storage.Storage, c *metrics.Collector) *APIServer {
	return NewAPI(s, c)
}

// Register attaches API endpoints (/create, /get/{id}, /settings, /isWritable, /healthz) to Gorilla Mux.
func (a *APIServer) Register(r *mux.Router) {
	r.HandleFunc("/create", a.handleCreate)
	r.HandleFunc("/get/{id}", a.handleRead)
	r.HandleFunc("/isWritable", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.HandleFunc("/settings", a.handleSettings).Methods(http.MethodGet)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

// errorResponse logs the internal error details with a tracking UUID and returns a sanitized JSON error.
func (a *apiServer) errorResponse(res http.ResponseWriter, status int, err error, desc string) {
	errID := uuid.Must(uuid.NewV4()).String()

	if desc != "" {
		// No description: Nothing interesting for the server log
		logrus.WithField("err_id", errID).WithError(err).Error(desc)
	}

	a.jsonResponse(res, status, apiResponse{
		Error: errID,
	})
}

// handleCreate processes incoming secret creation requests (JSON or form-urlencoded), enforcing rate limits,
// maximum secret size limits, and total instance attachment storage caps.
func (a *apiServer) handleCreate(res http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	if !a.rateLimiter.Allow(clientIP) {
		a.collector.CountSecretCreateError("rate_limit_exceeded")
		a.errorResponse(res, http.StatusTooManyRequests, errors.New("rate limit exceeded"), "rate limit exceeded for "+clientIP)
		return
	}

	if cust.MaxSecretSize > 0 {
		// As a safeguard against HUGE payloads behind a misconfigured
		// proxy we take double the maximum secret size after which we
		// just close the read and cut the connection to the sender.
		r.Body = http.MaxBytesReader(res, r.Body, cust.MaxSecretSize*2)
	}

	var (
		expiry = cfg.SecretExpiry
		secret string
	)

	if !cust.DisableExpiryOverride {
		var err error
		if expiry, err = a.parseExpiryOverride(r, expiry); err != nil {
			a.collector.CountSecretCreateError(errorReasonInvalidExpiry)
			a.errorResponse(res, http.StatusBadRequest, err, "")
			return
		}
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		tmp := apiRequest{}
		if err := json.NewDecoder(r.Body).Decode(&tmp); err != nil {
			if _, ok := err.(*http.MaxBytesError); ok {
				a.collector.CountSecretCreateError(errorReasonSecretSize)
				// We don't do an error response here as the MaxBytesReader
				// automatically cuts the ResponseWriter and we simply cannot
				// answer them.
				return
			}

			a.collector.CountSecretCreateError(errorReasonInvalidJSON)
			a.errorResponse(res, http.StatusBadRequest, err, "decoding request body")
			return
		}
		secret = tmp.Secret
	} else {
		secret = r.FormValue("secret")
	}

	if secret == "" {
		a.collector.CountSecretCreateError(errorReasonSecretMissing)
		a.errorResponse(res, http.StatusBadRequest, errors.New("secret missing"), "")
		return
	}

	if cust.MaxSecretSize > 0 && len(secret) > int(cust.MaxSecretSize) {
		a.collector.CountSecretCreateError(errorReasonSecretSize)
		a.errorResponse(res, http.StatusBadRequest, errors.New("secret size exceeds maximum"), "")
		return
	}

	if cust.MaxAttachmentSizeTotal > 0 {
		currentBytes := a.storageBytes.Load()
		if currentBytes+int64(len(secret)) > cust.MaxAttachmentSizeTotal {
			a.collector.CountSecretCreateError(errorReasonSecretSize)
			a.errorResponse(res, http.StatusInsufficientStorage, errors.New("total instance attachment storage limit reached"), "storage limit reached")
			return
		}
	}

	id, err := a.store.Create(secret, time.Duration(expiry)*time.Second)
	if err != nil {
		a.collector.CountSecretCreateError(errorReasonStorageError)
		a.errorResponse(res, http.StatusInternalServerError, err, "creating secret")
		return
	}

	a.storageBytes.Add(int64(len(secret)))

	var expiresAt *time.Time
	if expiry > 0 {
		expiresAt = func(v time.Time) *time.Time { return &v }(time.Now().UTC().Add(time.Duration(expiry) * time.Second))
	}

	a.collector.CountSecretCreated()
	a.jsonResponse(res, http.StatusCreated, apiResponse{
		ExpiresAt: expiresAt,
		Success:   true,
		SecretID:  id,
	})
}

// handleRead retrieves and atomically destroys a stored secret by secret ID (one-time read).
func (a *apiServer) handleRead(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		a.errorResponse(res, http.StatusBadRequest, errors.New("id missing"), "")
		return
	}

	secret, err := a.store.ReadAndDestroy(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, storage.ErrSecretNotFound) {
			a.collector.CountSecretReadError(errorReasonSecretNotFound)
			status = http.StatusNotFound
		} else {
			a.collector.CountSecretReadError(errorReasonStorageError)
		}
		a.errorResponse(res, status, err, "reading & destroying secret")
		return
	}

	a.storageBytes.Add(-int64(len(secret)))

	a.collector.CountSecretRead()
	a.jsonResponse(res, http.StatusOK, apiResponse{
		Success: true,
		Secret:  secret,
	})
}

// handleSettings serves instance customization parameters to frontend and CLI clients.
func (a *apiServer) handleSettings(w http.ResponseWriter, _ *http.Request) {
	a.jsonResponse(w, http.StatusOK, cust)
}

// jsonResponse helper writes JSON encoded responses with no-store cache headers.
func (*apiServer) jsonResponse(res http.ResponseWriter, status int, response any) {
	res.Header().Set("Content-Type", "application/json")
	res.Header().Set("Cache-Control", "no-store, max-age=0")
	res.WriteHeader(status)

	if err := json.NewEncoder(res).Encode(response); err != nil {
		logrus.WithError(err).Error("encoding JSON response")
		http.Error(res, `{"error":"could not encode response"}`, http.StatusInternalServerError)
	}
}

// parseExpiryOverride evaluates optional custom secret expiry request query parameters against server limits.
func (*apiServer) parseExpiryOverride(r *http.Request, expiry int64) (int64, error) {
	expiryValues, ok := r.URL.Query()["expire"]
	if !ok {
		return expiry, nil
	}

	ev, err := strconv.ParseInt(expiryValues[0], 10, 64)
	if err != nil {
		return 0, errors.New("invalid expiry")
	}

	if ev < 0 {
		return 0, errors.New("expiry must be greater than or equal to zero")
	}

	if ev > maxExpirySeconds {
		return 0, errors.New("expiry exceeds maximum duration")
	}

	if ev == 0 && cfg.SecretExpiry > 0 {
		return cfg.SecretExpiry, nil
	}

	if ev < expiry || cfg.SecretExpiry == 0 {
		return ev, nil
	}

	return expiry, nil
}
