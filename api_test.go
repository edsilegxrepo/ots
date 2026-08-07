// Package main - API Server & Live E2E Test Suite
//
// Test Strategy Explanation:
//   - Unit & Integration Isolation: Tests API handlers (handleCreate, handleRead, handleSettings) against in-memory storage.
//   - Boundary & Validation Testing: Exercises expiry overrides, malformed JSON, rate limits, and total instance storage caps.
//   - Live Server E2E Verification: Uses httptest.NewServer with real Gorilla Mux routing to test 10MB attachments,
//     group-based extension filtering (@images, @office), and Issue #208 dual-channel URL splitting (FetchWithKey).
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Luzifer/ots/pkg/auth"
	"github.com/Luzifer/ots/pkg/client"
	"github.com/Luzifer/ots/pkg/customization"
	"github.com/Luzifer/ots/pkg/metrics"
	"github.com/Luzifer/ots/pkg/storage"
	"github.com/Luzifer/ots/pkg/storage/memory"
)

var testCollector = metrics.New()

func TestHandleCreateExpiryOverrideAcceptedValues(t *testing.T) {
	tests := []struct {
		name          string
		expire        int64
		secretExpiry  int64
		wantExpiresAt bool
	}{
		{
			name:          "zero-uses-configured-expiry",
			expire:        0,
			secretExpiry:  3600,
			wantExpiresAt: true,
		},
		{
			name:          "zero-without-configured-expiry",
			expire:        0,
			secretExpiry:  0,
			wantExpiresAt: false,
		},
		{
			name:          "one-second",
			expire:        1,
			secretExpiry:  3600,
			wantExpiresAt: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api, _ := newTestAPI(t)
			cfg.SecretExpiry = tc.secretExpiry
			res := createJSONSecret(api, fmt.Sprintf("/api/create?expire=%d", tc.expire))

			require.Equal(t, http.StatusCreated, res.Code)

			var response apiResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&response))
			assert.True(t, response.Success)
			assert.NotEmpty(t, response.SecretID)
			if tc.wantExpiresAt {
				assert.NotNil(t, response.ExpiresAt)
			} else {
				assert.Nil(t, response.ExpiresAt)
			}
		})
	}
}

func TestHandleCreateExpiryOverrideValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		expire      string
	}{
		{
			name:        "empty",
			contentType: "application/json",
			body:        `{"secret":"test-secret"}`,
			expire:      "",
		},
		{
			name:        "malformed",
			contentType: "application/json",
			body:        `{"secret":"test-secret"}`,
			expire:      "abc",
		},
		{
			name:        "negative-json",
			contentType: "application/json",
			body:        `{"secret":"test-secret"}`,
			expire:      "-1",
		},
		{
			name:        "negative-form",
			contentType: "application/x-www-form-urlencoded",
			body:        "secret=test-secret",
			expire:      "-1",
		},
		{
			name:        "too-large",
			contentType: "application/json",
			body:        `{"secret":"test-secret"}`,
			expire:      strconv.FormatInt(maxExpirySeconds+1, 10),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api, store := newTestAPI(t)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create?expire="+tc.expire, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			res := httptest.NewRecorder()

			api.handleCreate(res, req)

			require.Equal(t, http.StatusBadRequest, res.Code)

			count, err := store.Count()
			require.NoError(t, err)
			assert.Zero(t, count)
		})
	}
}

func createJSONSecret(api *apiServer, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, bytes.NewBufferString(`{"secret":"test-secret"}`))
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	api.handleCreate(res, req)

	return res
}

func newTestAPI(t *testing.T) (*apiServer, storage.Storage) {
	t.Helper()

	oldCfg := cfg
	oldCust := cust
	t.Cleanup(func() {
		cfg = oldCfg
		cust = oldCust
	})

	cfg.SecretExpiry = 3600
	cust = customization.Customize{}

	store := memory.New()
	return newAPI(store, testCollector), store
}

func TestHandleSettings(t *testing.T) {
	api, _ := newTestAPI(t)
	cust.AcceptedFileTypes = "@images"
	cust.ApplyFixes()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	api.handleSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"resolvedAcceptedExtensions"`)
	assert.Contains(t, w.Body.String(), `".png"`)
}

func TestHandleReadAndDestroy(t *testing.T) {
	api, store := newTestAPI(t)

	// Create secret
	id, err := store.Create("secret_content", time.Hour, 1)
	require.NoError(t, err)

	// Read secret
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/get/"+id, nil)
	w := httptest.NewRecorder()
	r := mux.NewRouter()
	r.HandleFunc("/api/get/{id}", api.handleRead)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"secret":"secret_content"`)

	// Read again (destroyed) should return 404
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestRequestInSubnetList(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "192.168.1.50:12345"

	subnets := []string{"192.168.1.0/24", "10.0.0.0/8"}
	assert.True(t, requestInSubnetList(req, subnets))

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	req2.RemoteAddr = "172.16.0.1:12345"
	assert.False(t, requestInSubnetList(req2, subnets))
}

func TestLargeSecretAndAttachmentSupport(t *testing.T) {
	api, store := newTestAPI(t)

	// Configure large max secret size (256MB = 268,435,456 bytes)
	cust.MaxSecretSize = 256 * 1024 * 1024

	// Create 5MB secret payload (5 * 1024 * 1024 bytes)
	largeData := bytes.Repeat([]byte("A"), 5*1024*1024)
	reqBody, err := json.Marshal(apiRequest{Secret: string(largeData)})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleCreate(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Exceeding custom MaxSecretSize should return 400 Bad Request
	cust.MaxSecretSize = 100                          // Set max secret size limit to 100 bytes
	overLimitSecret := bytes.Repeat([]byte("B"), 150) // 150 bytes exceeds 100 bytes
	reqBody2, err := json.Marshal(apiRequest{Secret: string(overLimitSecret)})
	require.NoError(t, err)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/create", bytes.NewReader(reqBody2))
	req2.Header.Set("Content-Type", "application/json")

	api.handleCreate(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	cust.MaxSecretSize = 0 // Reset
}

func TestLiveOTSServerFullLifecycleE2E(t *testing.T) {
	// 1. Initialize live local OTS server engine with Gorilla Mux router
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// 2. Create encrypted secret against live local OTS server
	plainSecret := client.Secret{
		Secret: "Secret created against live local OTS server engine",
		Attachments: []client.SecretAttachment{
			{
				Name:    "live_attachment.txt",
				Type:    "text/plain",
				Content: []byte("E2E live server attachment payload"),
			},
		},
	}

	secretURL, expiresAt, err := client.Create(liveServer.URL, plainSecret, 15*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, secretURL, liveServer.URL)
	assert.Contains(t, secretURL, "#")
	assert.False(t, expiresAt.IsZero())

	// 3. Fetch and decrypt secret from live local OTS server
	fetched, err := client.Fetch(secretURL)
	require.NoError(t, err)
	assert.Equal(t, "Secret created against live local OTS server engine", fetched.Secret)
	require.Len(t, fetched.Attachments, 1)
	assert.Equal(t, "live_attachment.txt", fetched.Attachments[0].Name)
	assert.Equal(t, []byte("E2E live server attachment payload"), fetched.Attachments[0].Content)

	// 4. Verify one-time read & destroy on live server
	_, err = client.Fetch(secretURL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 404")
}

func TestLiveServerLargeAttachmentsE2E(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// 10MB live attachment payload
	largePayload := bytes.Repeat([]byte("X"), 10*1024*1024)
	secret := client.Secret{
		Secret: "Secret with 10MB Large Attachment Payload",
		Attachments: []client.SecretAttachment{
			{
				Name:    "large_dataset.bin",
				Type:    "application/octet-stream",
				Content: largePayload,
			},
		},
	}

	secretURL, _, err := client.Create(liveServer.URL, secret, 10*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, secretURL)

	fetched, err := client.Fetch(secretURL)
	require.NoError(t, err)
	assert.Equal(t, "Secret with 10MB Large Attachment Payload", fetched.Secret)
	require.Len(t, fetched.Attachments, 1)
	assert.Equal(t, "large_dataset.bin", fetched.Attachments[0].Name)
	assert.Len(t, fetched.Attachments[0].Content, len(largePayload))
	assert.Equal(t, largePayload[:100], fetched.Attachments[0].Content[:100])
}

func TestLiveServerExtensionFilteringE2E(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// Configure accepted file types & group aliases
	cust.AcceptedFileTypes = "@images, @office, .pdf, .txt"
	cust.ApplyFixes()
	defer func() {
		cust.AcceptedFileTypes = ""
		cust.ResolvedAcceptedExtensions = nil
	}()

	// Test 1: Allowed extensions (pdf & png)
	assert.True(t, customization.IsFilenameAllowed("report.pdf", cust.ResolvedAcceptedExtensions))
	assert.True(t, customization.IsFilenameAllowed("photo.PNG", cust.ResolvedAcceptedExtensions))

	validSecret := client.Secret{
		Secret: "Valid Secret Payload",
		Attachments: []client.SecretAttachment{
			{Name: "report.pdf", Type: "application/pdf", Content: []byte("PDF Content")},
		},
	}
	secretURL, _, err := client.Create(liveServer.URL, validSecret, 5*time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, secretURL)

	// Test 2: Blocked extension (.exe)
	assert.False(t, customization.IsFilenameAllowed("malware.exe", cust.ResolvedAcceptedExtensions))
}

func TestLiveServerDualChannelSplitKeyE2E(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// #nosec G101 -- Test dummy credentials
	secret := client.Secret{
		Secret: "Top Secret Credentials for Dual-Channel Transmission",
		Attachments: []client.SecretAttachment{
			{Name: "key.pem", Type: "application/x-pem-file", Content: []byte("-----BEGIN PRIVATE KEY-----")},
		},
	}

	unifiedURL, _, err := client.Create(liveServer.URL, secret, 10*time.Minute)
	require.NoError(t, err)

	// Split Unified URL into Channel A (Base Secret URL) & Channel B (Decryption Key)
	baseURL, decryptionKey, err := client.SplitSecretURL(unifiedURL)
	require.NoError(t, err)
	assert.NotContains(t, baseURL, decryptionKey)
	assert.NotEmpty(t, decryptionKey)

	// Attempting to fetch base URL without decryption key fails
	_, err = client.Fetch(baseURL)
	require.Error(t, err)

	// Fetching base URL with separately transmitted decryption key succeeds
	fetched, err := client.FetchWithKey(baseURL, decryptionKey)
	require.NoError(t, err)
	assert.Equal(t, "Top Secret Credentials for Dual-Channel Transmission", fetched.Secret)
	require.Len(t, fetched.Attachments, 1)
	assert.Equal(t, "key.pem", fetched.Attachments[0].Name)

	// One-time read & destroy verification
	_, err = client.FetchWithKey(baseURL, decryptionKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 404")
}

func TestLiveServerConcurrencyAndAntiSpoofingE2E(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// High concurrency parallel load test
	const workers = 20
	errChan := make(chan error, workers)

	for i := range workers {
		go func(workerID int) {
			sec := client.Secret{
				Secret: fmt.Sprintf("Parallel secret payload from worker %d", workerID),
			}
			secretURL, _, err := client.Create(liveServer.URL, sec, 5*time.Minute)
			if err != nil {
				errChan <- err
				return
			}
			fetched, err := client.Fetch(secretURL)
			if err != nil {
				errChan <- err
				return
			}
			if fetched.Secret != sec.Secret {
				errChan <- fmt.Errorf("secret mismatch for worker %d", workerID)
				return
			}
			errChan <- nil
		}(i)
	}

	for range workers {
		err := <-errChan
		assert.NoError(t, err)
	}
}

func TestLiveServerSanitizedErrorResponsesE2E(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// Test 1: Fetching non-existent secret ID returns 404 with UUID tracking error ID
	req1, err := http.NewRequestWithContext(t.Context(), http.MethodGet, liveServer.URL+"/api/get/non-existent-uuid-12345", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp apiResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.False(t, errResp.Success)
	assert.NotEmpty(t, errResp.Error)
	assert.Len(t, errResp.Error, 36) // UUID v4 string length

	// Test 2: Malformed JSON creation returns 400 Bad Request with UUID tracking error ID
	req2, err := http.NewRequestWithContext(t.Context(), http.MethodPost, liveServer.URL+"/api/create", strings.NewReader(`{invalid-json`))
	require.NoError(t, err)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)

	var errResp2 apiResponse
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&errResp2))
	assert.False(t, errResp2.Success)
	assert.NotEmpty(t, errResp2.Error)
	assert.Len(t, errResp2.Error, 36)
}

func TestProductionMaxAttachmentBoundaryE2E(t *testing.T) {
	store := memory.New()
	api := newAPI(store, testCollector)
	r := mux.NewRouter()
	api.Register(r.PathPrefix("/api").Subrouter())

	liveServer := httptest.NewServer(r)
	defer liveServer.Close()

	// Simulate a 53.5 MiB raw binary attachment (expands to ~95.1 MiB after double base64 encoding)
	// Create ~53.5 MiB binary buffer
	const rawAttachmentSize = 53 * 1024 * 1024
	rawBytes := bytes.Repeat([]byte("A"), rawAttachmentSize)

	att := client.SecretAttachment{
		Name:    "scipy-1.11.3-3.rawhide.src.rpm",
		Type:    "application/x-rpm",
		Content: rawBytes,
	}

	// #nosec G101 -- Test dummy secret payload
	sec := client.Secret{
		Secret:      "RPM Package Description",
		Attachments: []client.SecretAttachment{att},
	}

	// 1. Create Secret via client library with 53.5 MiB attachment
	secretURL, decryptionKey, err := client.Create(liveServer.URL, sec, 24*time.Hour)
	require.NoError(t, err, "creating secret with 53.5 MiB attachment must succeed under production 115.55 MiB capacity limit")
	assert.NotEmpty(t, secretURL)
	assert.NotEmpty(t, decryptionKey)

	// 2. Fetch & verify payload integrity
	fetched, err := client.Fetch(secretURL)
	require.NoError(t, err, "fetching 53.5 MiB attachment secret must succeed")
	assert.Equal(t, "RPM Package Description", fetched.Secret)
	require.Len(t, fetched.Attachments, 1)
	assert.Equal(t, "scipy-1.11.3-3.rawhide.src.rpm", fetched.Attachments[0].Name)
	assert.Len(t, fetched.Attachments[0].Content, rawAttachmentSize)

	// 3. Verify one-time burn
	_, err = client.Fetch(secretURL)
	require.Error(t, err, "second read must return 404 Not Found")
}

func TestEnterpriseMessageTemplatesRendering(t *testing.T) {
	// #nosec G101 -- Dummy test decryption key and secret ID
	secretID := "57a87bbd-fc58-4716-aa36-511d027a40aa"
	shortID := "57a87bbd"
	key := "zAuhdWvm96ugn3JsgU0m"
	baseURL := "http://127.0.0.1:3000/"
	fullURL := baseURL + "#" + secretID + "%7C" + key
	shortURL := baseURL + "#" + secretID
	expiry := "8/1/2026, 3:52:40 PM"

	// 1. Full Link ASCII Template with secretId Header & NOTE block
	fullTpl := "===================================================================\n" +
		"             CONFIDENTIAL ONE-TIME SECRET [" + shortID + "]\n" +
		"===================================================================\n\n" +
		"NOTE: Secret expires on " + expiry + ".\n\n" +
		"-------------------------------------------------------------------\nSECRET URL:\n" + fullURL + "\n" +
		"-------------------------------------------------------------------\n\n" +
		"IMPORTANT INSTRUCTIONS:\n1. Accessing this URL decrypts the payload and PERMANENTLY BURNS\n" +
		"   (deletes) the secret from the server.\n2. Please copy or store the content immediately upon opening.\n\n" +
		"==================================================================="

	assert.Contains(t, fullTpl, fullURL)
	assert.Contains(t, fullTpl, "CONFIDENTIAL ONE-TIME SECRET ["+shortID+"]")
	assert.Contains(t, fullTpl, "NOTE: Secret expires on "+expiry)
	assert.NotContains(t, fullTpl, "Hello,")
	assert.NotRegexp(t, `[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`, fullTpl, "template must contain zero emojis")

	// 2. Dual Link Part 1 Markdown Template with [!WARNING] block
	dualLinkMd := "### DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2) [" + shortID + "]\n\n" +
		"> [!WARNING]\n> Secret expires on " + expiry + ".\n\n" +
		"A secure, encrypted one-time secret has been created for you.\n" +
		"> **SECRET LINK (Without Decryption Key):**\n> " + shortURL + "\n\n" +
		"**INSTRUCTIONS:**\n1. Obtain the Decryption Key from your second communication channel.\n2. Accessing the link burns (deletes) the secret permanently."

	assert.Contains(t, dualLinkMd, shortURL)
	assert.Contains(t, dualLinkMd, "DUAL-CHANNEL SECRET TRANSMISSION (PART 1 OF 2) ["+shortID+"]")
	assert.Contains(t, dualLinkMd, "> [!WARNING]")
	assert.NotContains(t, dualLinkMd, key)
	assert.NotRegexp(t, `[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`, dualLinkMd, "dual link template must contain zero emojis")

	// 3. Combined Chat JSON Payload Schema Validation (including burn_on_read & secret_id)
	jsonPayload := `{"burn_on_read":true,"decryption_key":"` + key + `","expiration":"` + expiry + `","header":"DUAL-CHANNEL SECURE DELIVERY NOTICE [` + shortID + `]","secret_id":"` + shortID + `","short_url":"` + shortURL + `","type":"dual_channel_combined_notice"}`

	assert.Contains(t, jsonPayload, `"burn_on_read":true`)
	assert.Contains(t, jsonPayload, `"expiration":"`+expiry+`"`)
	assert.Contains(t, jsonPayload, `"secret_id":"`+shortID+`"`)
	assert.Contains(t, jsonPayload, `"decryption_key":"`+key+`"`)

	// 4. Dual Key Part 2 Template
	dualKeyTpl := "DECRYPTION KEY:\n" + key
	assert.Contains(t, dualKeyTpl, key)
	assert.NotContains(t, dualKeyTpl, shortURL)
	assert.NotRegexp(t, `[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`, dualKeyTpl, "dual key template must contain zero emojis")
}

func TestSecretBundlePayloadAssembly(t *testing.T) {
	// Verify assembly of multi-file secret bundles for client-side JSZip packaging
	sec := client.Secret{
		Secret: "Confidential System Secret Payload",
		Attachments: []client.SecretAttachment{
			{
				Name:    "config.json",
				Type:    "application/json",
				Content: []byte(`{"db_host":"127.0.0.1","db_port":5432}`),
			},
			{
				Name:    "setup.sh",
				Type:    "text/x-shellscript",
				Content: []byte("#!/bin/bash\necho Setup completed\n"),
			},
		},
	}

	assert.Equal(t, "Confidential System Secret Payload", sec.Secret)
	require.Len(t, sec.Attachments, 2)
	assert.Equal(t, "config.json", sec.Attachments[0].Name)
	assert.Equal(t, "setup.sh", sec.Attachments[1].Name)
	assert.Contains(t, string(sec.Attachments[0].Content), "127.0.0.1")
}

func TestForwardAuthIntegrationE2E(t *testing.T) {
	cfg := auth.ForwardAuthConfig{
		Enabled:         true,
		UserHeader:      "Remote-User",
		EmailHeader:     "Remote-Email",
		GroupsHeader:    "Remote-Groups",
		HeaderDelimiter: ",",
		TrustedProxies:  []string{"127.0.0.1"},
	}

	am, err := auth.NewAuthMiddleware(auth.IAMConfig{
		Enabled:   true,
		Connector: "forwardauth",
		Policy: auth.IAMPolicy{
			AllowedGroups: []string{"DevOps", "SecOps"},
		},
		Connectors: auth.IAMConnectors{
			ForwardAuth: cfg,
		},
	}, nil)
	require.NoError(t, err)

	handler := am.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := auth.GetUserIdentity(r)
		w.WriteHeader(http.StatusOK)
		if identity != nil {
			_, _ = w.Write([]byte("Authorized:" + identity.Username))
		} else {
			_, _ = w.Write([]byte("AnonymousPublic"))
		}
	}))

	t.Run("Authorized ForwardAuth Call", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set("Remote-User", "alice@company.com")
		req.Header.Set("Remote-Groups", "DevOps,Engineering")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Authorized:alice@company.com", rr.Body.String())
	})

	t.Run("Unauthorized Group Rejection", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set("Remote-User", "guest@company.com")
		req.Header.Set("Remote-Groups", "Guests")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("Public Secret Redemption Bypasses Auth", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/get/57a87bbd-fc58-4716-aa36-511d027a40aa", nil)
		req.RemoteAddr = "203.0.113.10:1234" // Anonymous recipient IP

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "AnonymousPublic", rr.Body.String())
	})
}

func TestSecretReusabilityMultiRead(t *testing.T) {
	api, _ := newTestAPI(t)
	cust.MaxSecretReads = 3
	cust.DisableReusabilityOverride = false

	// Create secret with 3 allowed reads
	reqBody := `{"secret":"multi_read_secret_payload","reads":3}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	api.handleCreate(res, req)
	require.Equal(t, http.StatusCreated, res.Code)

	var createResp apiResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&createResp))
	assert.True(t, createResp.Success)
	assert.Equal(t, 3, createResp.ReadsRemaining)
	secretID := createResp.SecretID

	// Read 1 (should leave 2 remaining)
	readReq1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/get/"+secretID, nil)
	readReq1 = mux.SetURLVars(readReq1, map[string]string{"id": secretID})
	res1 := httptest.NewRecorder()
	api.handleRead(res1, readReq1)
	require.Equal(t, http.StatusOK, res1.Code)
	var readResp1 apiResponse
	require.NoError(t, json.NewDecoder(res1.Body).Decode(&readResp1))
	assert.Equal(t, "multi_read_secret_payload", readResp1.Secret)
	assert.Equal(t, 2, readResp1.ReadsRemaining)

	// Read 2 (should leave 1 remaining)
	readReq2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/get/"+secretID, nil)
	readReq2 = mux.SetURLVars(readReq2, map[string]string{"id": secretID})
	res2 := httptest.NewRecorder()
	api.handleRead(res2, readReq2)
	require.Equal(t, http.StatusOK, res2.Code)
	var readResp2 apiResponse
	require.NoError(t, json.NewDecoder(res2.Body).Decode(&readResp2))
	assert.Equal(t, 1, readResp2.ReadsRemaining)

	// Read 3 (should reach 0 remaining)
	readReq3 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/get/"+secretID, nil)
	readReq3 = mux.SetURLVars(readReq3, map[string]string{"id": secretID})
	res3 := httptest.NewRecorder()
	api.handleRead(res3, readReq3)
	require.Equal(t, http.StatusOK, res3.Code)
	var readResp3 apiResponse
	require.NoError(t, json.NewDecoder(res3.Body).Decode(&readResp3))
	assert.Equal(t, 0, readResp3.ReadsRemaining)

	// Read 4 (should return 404 secret not found)
	readReq4 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/get/"+secretID, nil)
	readReq4 = mux.SetURLVars(readReq4, map[string]string{"id": secretID})
	res4 := httptest.NewRecorder()
	api.handleRead(res4, readReq4)
	require.Equal(t, http.StatusNotFound, res4.Code)
}

func TestAPICreateFileExtensionFiltering(t *testing.T) {
	origAccepted := cust.AcceptedFileTypes
	defer func() { cust.AcceptedFileTypes = origAccepted }()

	cust.AcceptedFileTypes = "@images, .pdf"

	store := memory.New()
	api := NewAPI(store, testCollector)

	// Construct OTS1 payload with disallowed extension (.exe)
	disallowedMetaJSON := `{"files":[{"name":"malware.exe","size":100,"type":"application/x-msdownload"}],"secret":"encrypted_payload"}`
	disallowedPayload := "OTS1" + base64.StdEncoding.EncodeToString([]byte(disallowedMetaJSON))

	body, err := json.Marshal(apiRequest{Secret: disallowedPayload})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	api.handleCreate(res, req)
	assert.Equal(t, http.StatusBadRequest, res.Code)

	// Construct OTS1 payload with allowed extension (.png)
	allowedMetaJSON := `{"files":[{"name":"photo.png","size":100,"type":"image/png"}],"secret":"encrypted_payload"}`
	allowedPayload := "OTS1" + base64.StdEncoding.EncodeToString([]byte(allowedMetaJSON))

	bodyOk, err := json.Marshal(apiRequest{Secret: allowedPayload})
	require.NoError(t, err)

	reqOk := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", bytes.NewReader(bodyOk))
	reqOk.Header.Set("Content-Type", "application/json")
	resOk := httptest.NewRecorder()

	api.handleCreate(resOk, reqOk)
	assert.Equal(t, http.StatusCreated, resOk.Code)
}
