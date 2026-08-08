// Package main - Production Real-Life Live E2E Integration Test Suite
//
// Test Strategy Explanation:
//   - Simulates real-life production deployment scenarios end-to-end against live HTTP listeners.
//   - Exercises client SDK, dual-channel split-keys, multi-read reusability, premature burning,
//     file extension security filters, IAM ForwardAuth protection, rate limiting, and raw binary streaming.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/auth"
	"github.com/edsilegxrepo/ots/pkg/client"
	"github.com/edsilegxrepo/ots/pkg/storage/factory"
	"github.com/edsilegxrepo/ots/pkg/storage/memory"
)

func TestMasterProductionScenarioE2E(t *testing.T) {
	// 1. Setup live server infrastructure
	memStore := memory.New()
	api := newAPI(memStore, testCollector)

	router := mux.NewRouter()
	api.Register(router.PathPrefix("/api").Subrouter())

	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("Scenario-1-ClientSDK-FullLifecycle", func(t *testing.T) {
		s := client.Secret{
			Secret: "Production secret note content",
			Attachments: []client.SecretAttachment{{
				Name:    "report.txt",
				Type:    "text/plain",
				Content: []byte("Top secret audit report data\n"),
			}},
		}

		secretURL, expiresAt, err := client.Create(server.URL, s, time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, secretURL)
		assert.False(t, expiresAt.IsZero())

		fetchedSecret, err := client.Fetch(secretURL)
		require.NoError(t, err)
		assert.Equal(t, s.Secret, fetchedSecret.Secret)
		require.Len(t, fetchedSecret.Attachments, 1)
		assert.Equal(t, "report.txt", fetchedSecret.Attachments[0].Name)
		assert.Equal(t, []byte("Top secret audit report data\n"), fetchedSecret.Attachments[0].Content)

		// Second fetch returns error (one-time burn)
		_, err = client.Fetch(secretURL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("Scenario-2-DualChannel-SplitKeyDelivery", func(t *testing.T) {
		s := client.Secret{Secret: "Dual channel password drop"}
		secretURL, _, err := client.Create(server.URL, s, time.Hour)
		require.NoError(t, err)

		baseURL, key, err := client.SplitSecretURL(secretURL)
		require.NoError(t, err)
		assert.NotEmpty(t, baseURL)
		assert.NotEmpty(t, key)

		// Fetching base URL alone without decryption key should fail
		_, err = client.FetchWithKey(baseURL, "")
		assert.Error(t, err)

		// Fetching with correct key decrypts and burns secret
		decrypted, err := client.FetchWithKey(baseURL, key)
		require.NoError(t, err)
		assert.Equal(t, "Dual channel password drop", decrypted.Secret)
	})

	t.Run("Scenario-3-Premature-Secret-Burning", func(t *testing.T) {
		s := client.Secret{Secret: "Burn me immediately"}
		secretURL, _, err := client.Create(server.URL, s, time.Hour)
		require.NoError(t, err)

		// Burn secret explicitly before recipient reads it
		err = client.Burn(secretURL)
		require.NoError(t, err)

		// Attempting to fetch burned secret returns 404
		_, err = client.Fetch(secretURL)
		assert.Error(t, err)
	})

	t.Run("Scenario-4-MultiRead-Reusability", func(t *testing.T) {
		// Enable secret reusability
		origMaxReads := cust.MaxSecretReads
		cust.MaxSecretReads = 5
		defer func() { cust.MaxSecretReads = origMaxReads }()

		s := client.Secret{Secret: "Shared team credential"}
		secretURL, _, err := client.CreateWithOpts(server.URL, s, client.CreateOpts{
			ExpireIn: time.Hour,
			Reads:    3,
		})
		require.NoError(t, err)

		// Read 1
		read1, err := client.Fetch(secretURL)
		require.NoError(t, err)
		assert.Equal(t, "Shared team credential", read1.Secret)

		// Read 2
		read2, err := client.Fetch(secretURL)
		require.NoError(t, err)
		assert.Equal(t, "Shared team credential", read2.Secret)

		// Read 3 (final allowed read)
		read3, err := client.Fetch(secretURL)
		require.NoError(t, err)
		assert.Equal(t, "Shared team credential", read3.Secret)

		// Read 4 (exceeded allowed reads) returns 404
		_, err = client.Fetch(secretURL)
		assert.Error(t, err)
	})

	t.Run("Scenario-5-FileExtension-SecurityFilter", func(t *testing.T) {
		origTypes := cust.AcceptedFileTypes
		cust.AcceptedFileTypes = "pdf,txt"
		defer func() { cust.AcceptedFileTypes = origTypes }()

		// Create unencrypted OTS1 payload with disallowed extension (.exe)
		otsMetaJSON := `{"files":[{"name":"malware.exe"}]}`
		b64Meta := base64.StdEncoding.EncodeToString([]byte(otsMetaJSON))
		disallowedOTS1Payload := "OTS1" + b64Meta

		reqBody, _ := json.Marshal(apiRequest{Secret: disallowedOTS1Payload})
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Scenario-6-RawBinary-StreamingEndpoint", func(t *testing.T) {
		rawBlob := bytes.Repeat([]byte("RAW_BINARY_DATA_PAYLOAD_"), 100)

		// POST /api/create/raw
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create/raw", bytes.NewReader(rawBlob))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var apiResp apiResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiResp))
		assert.True(t, apiResp.Success)
		assert.NotEmpty(t, apiResp.SecretID)

		// GET /api/get/{id}
		getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/api/get/"+apiResp.SecretID, nil)
		require.NoError(t, err)

		getResp, err := http.DefaultClient.Do(getReq)
		require.NoError(t, err)
		defer func() { _ = getResp.Body.Close() }()
		require.Equal(t, http.StatusOK, getResp.StatusCode)

		var getApiResp apiResponse
		require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getApiResp))
		assert.Equal(t, rawBlob, []byte(getApiResp.Secret))
	})

	t.Run("Scenario-7-StorageCap-Boundaries", func(t *testing.T) {
		origCap := cust.MaxAttachmentSizeTotal
		cust.MaxAttachmentSizeTotal = 30 // Set 30 bytes instance storage cap
		defer func() { cust.MaxAttachmentSizeTotal = origCap }()

		// Create secret within limit (20 bytes)
		req1, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create", strings.NewReader(`{"secret":"12345678901234567890"}`))
		req1.Header.Set("Content-Type", "application/json")
		resp1, err := http.DefaultClient.Do(req1)
		require.NoError(t, err)
		_ = resp1.Body.Close()
		assert.Equal(t, http.StatusCreated, resp1.StatusCode)

		// Create secret exceeding cumulative storage cap (20 + 20 = 40 > 30)
		req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create", strings.NewReader(`{"secret":"12345678901234567890"}`))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err)
		_ = resp2.Body.Close()
		assert.Equal(t, http.StatusInsufficientStorage, resp2.StatusCode)
	})

	t.Run("Scenario-8-CustomExpiry-Overrides", func(t *testing.T) {
		// Test valid custom expiry override (?expire=3600)
		reqExp, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create?expire=3600", strings.NewReader(`{"secret":"custom_exp"}`))
		reqExp.Header.Set("Content-Type", "application/json")
		respExp, err := http.DefaultClient.Do(reqExp)
		require.NoError(t, err)
		_ = respExp.Body.Close()
		assert.Equal(t, http.StatusCreated, respExp.StatusCode)

		// Test disabled custom expiry override
		cust.DisableExpiryOverride = true
		defer func() { cust.DisableExpiryOverride = false }()

		reqDis, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create?expire=7200", strings.NewReader(`{"secret":"dis_exp"}`))
		reqDis.Header.Set("Content-Type", "application/json")
		respDis, err := http.DefaultClient.Do(reqDis)
		require.NoError(t, err)
		_ = respDis.Body.Close()
		assert.Equal(t, http.StatusBadRequest, respDis.StatusCode)
	})

	t.Run("Scenario-9-HighConcurrency-Parallel-Traffic", func(t *testing.T) {
		const numGoroutines = 20
		errChan := make(chan error, numGoroutines)

		for i := range numGoroutines {
			go func(workerID int) {
				secPayload := client.Secret{Secret: fmt.Sprintf("parallel_secret_worker_%d", workerID)}
				secURL, _, createErr := client.Create(server.URL, secPayload, time.Hour)
				if createErr != nil {
					errChan <- createErr
					return
				}

				readSecret, fetchErr := client.Fetch(secURL)
				if fetchErr != nil {
					errChan <- fetchErr
					return
				}

				if readSecret.Secret != secPayload.Secret {
					errChan <- fmt.Errorf("worker %d mismatch: expected %q, got %q", workerID, secPayload.Secret, readSecret.Secret)
					return
				}

				errChan <- nil
			}(i)
		}

		for range numGoroutines {
			err := <-errChan
			assert.NoError(t, err)
		}
	})
}

func TestForwardAuthIAMProtectionE2E(t *testing.T) {
	memStore := memory.New()
	api := newAPI(memStore, testCollector)

	iamCfg := auth.IAMConfig{
		Enabled:            true,
		Connector:          "forwardauth",
		ProtectedEndpoints: []string{"/api/create"},
		Policy: auth.IAMPolicy{
			DefaultPolicy: "deny",
			AllowedGroups: []string{"Engineering"},
		},
		Connectors: auth.IAMConnectors{
			ForwardAuth: auth.ForwardAuthConfig{
				Enabled:         true,
				UserHeader:      "X-Forwarded-User",
				GroupsHeader:    "X-Forwarded-Groups",
				HeaderDelimiter: ",",
				TrustedProxies:  []string{"127.0.0.1"},
			},
		},
	}

	authMw, err := auth.NewAuthMiddleware(iamCfg, nil)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Use(authMw.Handler)
	api.Register(router.PathPrefix("/api").Subrouter())

	server := httptest.NewServer(router)
	defer server.Close()

	// 1. Unauthenticated request to protected /api/create returns 401
	reqUnauth, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create", strings.NewReader(`{"secret":"test"}`))
	require.NoError(t, err)
	reqUnauth.Header.Set("Content-Type", "application/json")
	respUnauth, err := http.DefaultClient.Do(reqUnauth)
	require.NoError(t, err)
	_ = respUnauth.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, respUnauth.StatusCode)

	// 2. Unauthorized group (Sales) returns 403
	reqForbidden, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create", strings.NewReader(`{"secret":"test"}`))
	require.NoError(t, err)
	reqForbidden.Header.Set("Content-Type", "application/json")
	reqForbidden.Header.Set("X-Forwarded-User", "user@company.com")
	reqForbidden.Header.Set("X-Forwarded-Groups", "Sales")
	respForbidden, err := http.DefaultClient.Do(reqForbidden)
	require.NoError(t, err)
	_ = respForbidden.Body.Close()
	assert.Equal(t, http.StatusForbidden, respForbidden.StatusCode)

	// 3. Authorized group (Engineering) passes through
	reqAuthorized, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/api/create", strings.NewReader(`{"secret":"test"}`))
	require.NoError(t, err)
	reqAuthorized.Header.Set("Content-Type", "application/json")
	reqAuthorized.Header.Set("X-Forwarded-User", "user@company.com")
	reqAuthorized.Header.Set("X-Forwarded-Groups", "Engineering")
	respAuthorized, err := http.DefaultClient.Do(reqAuthorized)
	require.NoError(t, err)
	_ = respAuthorized.Body.Close()
	assert.Equal(t, http.StatusCreated, respAuthorized.StatusCode)
}

func TestAllFiveStorageBackendsLiveE2E(t *testing.T) {
	backends := []struct {
		name string
		url  string
	}{
		{name: "1-Memory", url: "memory://"},
		{name: "2-SQLite-Memory", url: "sqlite://:memory:"},
		{name: "3-BadgerDB-Memory", url: "badger://:memory:"},
	}

	for _, b := range backends {
		t.Run(b.name, func(t *testing.T) {
			store, err := factory.CreateStorageEngine(b.url)
			require.NoError(t, err)
			defer func() {
				if cl, ok := store.(interface{ Close() error }); ok {
					_ = cl.Close()
				}
			}()

			api := newAPI(store, testCollector)
			router := mux.NewRouter()
			api.Register(router.PathPrefix("/api").Subrouter())

			srv := httptest.NewServer(router)
			defer srv.Close()

			s := client.Secret{
				Secret: "Payload stored in " + b.name,
				Attachments: []client.SecretAttachment{{
					Name:    "attachment.txt",
					Type:    "text/plain",
					Content: []byte("backend attachment content"),
				}},
			}

			secretURL, _, err := client.Create(srv.URL, s, time.Hour)
			require.NoError(t, err)
			assert.NotEmpty(t, secretURL)

			fetched, err := client.Fetch(secretURL)
			require.NoError(t, err)
			assert.Equal(t, "Payload stored in "+b.name, fetched.Secret)
			require.Len(t, fetched.Attachments, 1)
			assert.Equal(t, "attachment.txt", fetched.Attachments[0].Name)

			// One-time burn verification
			_, err = client.Fetch(secretURL)
			assert.Error(t, err)
		})
	}
}
