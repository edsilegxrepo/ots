package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/storage/memory"
)

func TestBase64OptimizationPooledDecoding(t *testing.T) {
	encStd := "QmFzZTY0IE9wdGltaXphdGlvbiBaaWVyby1BbGxvY2F0aW9uIEJ1ZmZlciBQb29sIFVuaXQgVGVzdCBQYXlsb2Fk"

	decoded, err := DecodeBase64Pooled(encStd)
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "Base64 Optimization")
}

func TestRawBinaryStreamingEndpointE2E(t *testing.T) {
	store := memory.New()
	apiServer := NewAPI(store, testCollector)

	router := mux.NewRouter()
	apiServer.Register(router)

	testServer := httptest.NewServer(router)
	defer testServer.Close()

	// 1. Submit raw binary payload via POST /api/create/raw
	binaryContent := []byte{0x00, 0xFF, 0xDE, 0xAD, 0xBE, 0xEF, 0x42, 0x13, 0x37}
	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/create/raw", bytes.NewReader(binaryContent))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResp apiResponse
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err)
	assert.True(t, createResp.Success)
	assert.NotEmpty(t, createResp.SecretID)

	// 2. Fetch created secret via GET /api/get/{id}
	getResp, err := client.Get(testServer.URL + "/get/" + createResp.SecretID)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var readResp apiResponse
	err = json.NewDecoder(getResp.Body).Decode(&readResp)
	require.NoError(t, err)
	assert.True(t, readResp.Success)
	assert.NotEmpty(t, readResp.Secret)
}

func TestBinaryStorageNormalizationCrossBackends(t *testing.T) {
	store := memory.New()
	rawPayload := []byte("Raw Binary Storage Normalization Test Blob")

	// Store raw payload
	id, err := store.Create(rawPayload, 5*time.Minute, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Retrieve raw payload
	fetched, readsRemaining, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Equal(t, 0, readsRemaining)
	assert.Equal(t, rawPayload, fetched)

	// Verify one-time destruction
	_, _, err = store.ReadAndDestroy(id)
	assert.Error(t, err)
}
