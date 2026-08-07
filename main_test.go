package main

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/storage/memory"
)

func TestHandleRobotsDisabled(t *testing.T) {
	disable := true
	cust.DisableSearchIndex = &disable

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()

	handleRobots(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "noindex, nofollow, noarchive, nosnippet", res.Header.Get("X-Robots-Tag"))
	assert.Contains(t, w.Body.String(), "Disallow: /")
}

func TestHandleRobotsEnabled(t *testing.T) {
	disable := false
	cust.DisableSearchIndex = &disable

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()

	handleRobots(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Empty(t, res.Header.Get("X-Robots-Tag"))
	assert.Contains(t, w.Body.String(), "Allow: /")

	// Restore default
	disableDefault := true
	cust.DisableSearchIndex = &disableDefault
}

func TestRateLimiterIntegration(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	api.rateLimiter = newIPRateLimiter(2, 1*time.Minute)

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewBufferString(`{"secret":"hello"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.RemoteAddr = "10.0.0.1:12345"
	w1 := httptest.NewRecorder()
	api.handleCreate(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewBufferString(`{"secret":"hello2"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "10.0.0.1:12345"
	w2 := httptest.NewRecorder()
	api.handleCreate(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)

	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewBufferString(`{"secret":"hello3"}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.RemoteAddr = "10.0.0.1:12345"
	w3 := httptest.NewRecorder()
	api.handleCreate(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)
}

func TestCumulativeStorageCapIntegration(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)

	cust.MaxAttachmentSizeTotal = 30 // Limit cumulative storage to 30 bytes

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewBufferString(`{"secret":"123456789012345"}`)) // 15 bytes
	req1.Header.Set("Content-Type", "application/json")
	req1.RemoteAddr = "10.0.0.2:12345"
	w1 := httptest.NewRecorder()
	api.handleCreate(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code)

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewBufferString(`{"secret":"12345678901234567890"}`)) // 20 bytes (total would be 35 > 30)
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "10.0.0.2:12345"
	w2 := httptest.NewRecorder()
	api.handleCreate(w2, req2)
	require.Equal(t, http.StatusInsufficientStorage, w2.Code)

	cust.MaxAttachmentSizeTotal = 0 // Reset
}

func TestAssetDelivery(t *testing.T) {
	// Request without dot
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/nodot", nil)
	w1 := httptest.NewRecorder()
	assetDelivery(w1, req1)
	assert.Equal(t, http.StatusNotFound, w1.Code)

	// Request for non-existent file with dot
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/nonexistent.css", nil)
	w2 := httptest.NewRecorder()
	assetDelivery(w2, req2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestHandleRemoveAcceptEncoding(t *testing.T) {
	called := false
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Empty(t, r.Header.Get("Accept-Encoding"))
	})

	h := handleRemoveAcceptEncoding(dummy)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.True(t, called)
}

func TestGetStorageByType(t *testing.T) {
	sMem, err := getStorageByType("mem")
	require.NoError(t, err)
	assert.NotNil(t, sMem)

	sUnknown, err := getStorageByType("unknown")
	require.Error(t, err)
	assert.Nil(t, sUnknown)
}

func TestSRICache(t *testing.T) {
	cache := newSRICache()

	val, found := cache.Get("nonexistent")
	assert.False(t, found)
	assert.Empty(t, val)

	cache.Set("app.css", "sha384-dummyhash")
	val, found = cache.Get("app.css")
	assert.True(t, found)
	assert.Equal(t, "sha384-dummyhash", val)
}

func TestAPIRegister(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()

	api.Register(r)

	// Test healthz endpoint
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test isWritable endpoint
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/isWritable", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNoContent, w2.Code)
}

func TestHandleIndex(t *testing.T) {
	source, err := assets.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Skip("frontend/dist/index.html not embedded in test environment")
		return
	}
	indexTpl = template.Must(template.New("index.html").Funcs(tplFuncs).Parse(string(source)))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Header().Get("Content-Security-Policy"), "script-src")
	assert.Contains(t, w.Body.String(), "<html")
}

func TestUpdateStoredSecretsCount(t *testing.T) {
	store := memory.New()
	_, err := store.Create("secret_1", time.Hour, 1)
	require.NoError(t, err)

	updateStoredSecretsCount(store, testCollector)
}

func TestListenerHardening(t *testing.T) {
	// Test default :3000 hardens to 127.0.0.1:3000 when TLS disabled
	assert.Equal(t, "127.0.0.1:3000", hardenListener(":3000", false))

	// Test custom --listen 0.0.0.0:8080 is respected when TLS disabled (with warning log)
	assert.Equal(t, "0.0.0.0:8080", hardenListener("0.0.0.0:8080", false))

	// Test TLS enabled retains original binding
	assert.Equal(t, ":3000", hardenListener(":3000", true))
	assert.Equal(t, "0.0.0.0:3000", hardenListener("0.0.0.0:3000", true))
}

func TestLogFormatNDJSON(t *testing.T) {
	origFormat := cfg.LogFormat
	defer func() { cfg.LogFormat = origFormat }()

	cfg.LogFormat = "ndjson"
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})

	logrus.WithField("test_key", "test_val").Info("test NDJSON log entry")

	output := buf.String()
	assert.Contains(t, output, `"level":"info"`)
	assert.Contains(t, output, `"msg":"test NDJSON log entry"`)
	assert.Contains(t, output, `"test_key":"test_val"`)
}

func TestLogFilePathWriting(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_ots.log")

	// #nosec G302 -- Test log file creation
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	require.NoError(t, err)
	defer func() { _ = logFile.Close() }()

	logrus.SetOutput(logFile)
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	logrus.Info("written to file log")

	//nolint:gosec // Test log file reading from temporary directory
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"msg":"written to file log"`)
}
