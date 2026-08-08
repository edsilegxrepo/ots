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
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/edsilegxrepo/ots/pkg/customization"
	"github.com/edsilegxrepo/ots/pkg/metrics"
	"github.com/edsilegxrepo/ots/pkg/storage"
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
	Success        bool       `json:"success"`
	Error          string     `json:"error,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	ReadsRemaining int        `json:"reads_remaining,omitempty"`
	Secret         string     `json:"secret,omitempty"` //#nosec:G117 // This application works with secrets
	SecretID       string     `json:"secret_id,omitempty"`
}

// apiRequest models incoming JSON secret submission payloads.
type apiRequest struct {
	Reads  int    `json:"reads,omitempty"`
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

// Register attaches API endpoints (/create, /create/raw, /get/{id}, /burn/{id}, /settings, /isWritable, /healthz) to Gorilla Mux.
func (a *APIServer) Register(r *mux.Router) {
	r.HandleFunc("/create", a.handleCreate)
	r.HandleFunc("/create/raw", a.handleCreateRaw).Methods(http.MethodPost)
	r.HandleFunc("/get/{id}", a.handleRead)
	r.HandleFunc("/burn/{id}", a.handleBurn)
	r.HandleFunc("/isWritable", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.HandleFunc("/settings", a.handleSettings).Methods(http.MethodGet)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

// errorResponse logs the internal error details with a tracking UUID and returns a sanitized JSON error.
func (a *apiServer) errorResponse(res http.ResponseWriter, status int, err error, desc string) {
	errID := GenerateUUID()

	if desc != "" {
		// No description: Nothing interesting for the server log
		logrus.WithField("err_id", errID).WithError(err).Error(desc)
	}

	a.jsonResponse(res, status, apiResponse{
		Error: errID,
	})
}

// handleCreate processes incoming secret creation requests (JSON or form-urlencoded), enforcing rate limits,
// storage caps, maximum secret payload boundaries, and expiration lifetimes.
func (a *apiServer) handleCreate(res http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	if !a.rateLimiter.Allow(clientIP) {
		a.collector.CountSecretCreateError("rate_limit_exceeded")
		a.errorResponse(res, http.StatusTooManyRequests, errors.New("rate limit exceeded"), "rate limit exceeded for "+clientIP)
		return
	}

	var (
		err    error
		expiry int64
		secret string
	)

	if expiry, err = a.parseExpiryOverride(r, cfg.SecretExpiry); err != nil {
		a.collector.CountSecretCreateError(errorReasonInvalidExpiry)
		a.errorResponse(res, http.StatusBadRequest, err, "parsing secret expiry")
		return
	}

	if cust.DisableExpiryOverride && expiry != cfg.SecretExpiry {
		a.collector.CountSecretCreateError(errorReasonInvalidExpiry)
		a.errorResponse(res, http.StatusBadRequest, errors.New("custom expiry is disabled on this instance"), "")
		return
	}

	requestedReads := 1
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
		if tmp.Reads > 1 {
			requestedReads = tmp.Reads
		}
	} else {
		secret = r.FormValue("secret")
		if r.FormValue("reads") != "" {
			if parsed, parseErr := strconv.Atoi(r.FormValue("reads")); parseErr == nil && parsed > 1 {
				requestedReads = parsed
			}
		}
	}

	if requestedReads > 1 {
		if cust.MaxSecretReads == 0 || cust.DisableReusabilityOverride {
			a.collector.CountSecretCreateError(errorReasonInvalidJSON)
			a.errorResponse(res, http.StatusBadRequest, errors.New("secret reusability is disabled on this instance"), "")
			return
		}
		if requestedReads > cust.MaxSecretReads {
			a.collector.CountSecretCreateError(errorReasonInvalidJSON)
			a.errorResponse(res, http.StatusBadRequest, errors.New("requested reads exceed maximum allowed by server"), "")
			return
		}
	}

	if secret == "" {
		a.collector.CountSecretCreateError(errorReasonSecretMissing)
		a.errorResponse(res, http.StatusBadRequest, errors.New("secret missing"), "")
		return
	}

	// Backend API-Level File Extension Validation
	if cust.AcceptedFileTypes != "" {
		allowedExts := customization.ExpandAcceptedFileTypes(cust.AcceptedFileTypes, nil)
		if len(allowedExts) > 0 {
			if filenames := extractOTSAttachedFilenames(secret); len(filenames) > 0 {
				for _, fname := range filenames {
					if !customization.IsFilenameAllowed(fname, allowedExts) {
						a.collector.CountSecretCreateError("file_extension_not_allowed")
						a.errorResponse(res, http.StatusBadRequest, errors.New("file extension not allowed on this server"), "disallowed attachment extension: "+fname)
						return
					}
				}
			}
		}
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

	payloadBytes := []byte(secret)

	id, err := a.store.Create(payloadBytes, time.Duration(expiry)*time.Second, requestedReads)
	if err != nil {
		a.collector.CountSecretCreateError(errorReasonStorageError)
		a.errorResponse(res, http.StatusInternalServerError, err, "creating secret")
		return
	}

	a.storageBytes.Add(int64(len(payloadBytes)))

	var expiresAt *time.Time
	if expiry > 0 {
		expiresAt = func(v time.Time) *time.Time { return &v }(time.Now().UTC().Add(time.Duration(expiry) * time.Second))
	}

	a.collector.CountSecretCreated()
	a.jsonResponse(res, http.StatusCreated, apiResponse{
		ExpiresAt:      expiresAt,
		ReadsRemaining: requestedReads,
		Success:        true,
		SecretID:       id,
	})
}

// handleCreateRaw processes raw binary payload submissions (application/octet-stream) directly into storage.
func (a *apiServer) handleCreateRaw(res http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	if !a.rateLimiter.Allow(clientIP) {
		a.collector.CountSecretCreateError("rate_limit_exceeded")
		a.errorResponse(res, http.StatusTooManyRequests, errors.New("rate limit exceeded"), "rate limit exceeded for "+clientIP)
		return
	}

	expiry, err := a.parseExpiryOverride(r, cfg.SecretExpiry)
	if err != nil {
		a.collector.CountSecretCreateError(errorReasonInvalidExpiry)
		a.errorResponse(res, http.StatusBadRequest, err, "parsing secret expiry")
		return
	}

	maxReadSize := int64(64 * 1024 * 1024) // Default 64MB hard limit
	if cust.MaxSecretSize > 0 {
		maxReadSize = cust.MaxSecretSize
	}

	r.Body = http.MaxBytesReader(res, r.Body, maxReadSize)
	payloadBytes, err := io.ReadAll(r.Body)
	if err != nil || len(payloadBytes) == 0 {
		a.collector.CountSecretCreateError(errorReasonSecretMissing)
		a.errorResponse(res, http.StatusBadRequest, errors.New("raw payload missing or unreadable"), "")
		return
	}

	if cust.MaxSecretSize > 0 && int64(len(payloadBytes)) > cust.MaxSecretSize {
		a.collector.CountSecretCreateError(errorReasonSecretSize)
		a.errorResponse(res, http.StatusBadRequest, errors.New("secret size exceeds maximum"), "")
		return
	}

	if cust.MaxAttachmentSizeTotal > 0 {
		currentBytes := a.storageBytes.Load()
		if currentBytes+int64(len(payloadBytes)) > cust.MaxAttachmentSizeTotal {
			a.collector.CountSecretCreateError(errorReasonSecretSize)
			a.errorResponse(res, http.StatusInsufficientStorage, errors.New("total instance attachment storage limit reached"), "storage limit reached")
			return
		}
	}

	id, err := a.store.Create(payloadBytes, time.Duration(expiry)*time.Second, 1)
	if err != nil {
		a.collector.CountSecretCreateError(errorReasonStorageError)
		a.errorResponse(res, http.StatusInternalServerError, err, "creating raw secret")
		return
	}

	a.storageBytes.Add(int64(len(payloadBytes)))
	a.collector.CountSecretCreated()

	a.jsonResponse(res, http.StatusCreated, apiResponse{
		Success:  true,
		SecretID: id,
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

	payloadBytes, readsRemaining, err := a.store.ReadAndDestroy(id)
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

	if readsRemaining <= 0 {
		a.storageBytes.Add(-int64(len(payloadBytes)))
	}

	a.collector.CountSecretRead()
	a.jsonResponse(res, http.StatusOK, apiResponse{
		ReadsRemaining: readsRemaining,
		Success:        true,
		Secret:         string(payloadBytes),
	})
}

// handleBurn immediately destroys a stored secret by secret ID (early manual receiver burn).
func (a *apiServer) handleBurn(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		a.errorResponse(res, http.StatusBadRequest, errors.New("id missing"), "")
		return
	}

	payloadBytes, err := a.store.Purge(id)
	if err != nil {
		if errors.Is(err, storage.ErrSecretNotFound) {
			a.jsonResponse(res, http.StatusOK, apiResponse{
				ReadsRemaining: 0,
				Success:        true,
			})
			return
		}
		a.errorResponse(res, http.StatusInternalServerError, err, "purging secret")
		return
	}

	if len(payloadBytes) > 0 {
		a.storageBytes.Add(-int64(len(payloadBytes)))
	}

	a.jsonResponse(res, http.StatusOK, apiResponse{
		ReadsRemaining: 0,
		Success:        true,
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

type otsMetaPayload struct {
	Files []struct {
		Name string `json:"name"`
	} `json:"files"`
}

func extractOTSAttachedFilenames(secret string) []string {
	if !strings.HasPrefix(secret, "OTS1") {
		return nil
	}

	b64Data := strings.TrimPrefix(secret, "OTS1")
	rawJSON, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil
	}

	var meta otsMetaPayload
	if err := json.Unmarshal(rawJSON, &meta); err != nil {
		return nil
	}

	var names []string
	for _, f := range meta.Files {
		if f.Name != "" {
			names = append(names, f.Name)
		}
	}
	return names
}
