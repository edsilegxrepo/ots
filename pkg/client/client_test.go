package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePassword(t *testing.T) {
	pass, err := genPass()
	require.NoError(t, err)

	assert.Len(t, pass, PasswordLength)
	assert.Regexp(t, `^[0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ]+$`, pass)
}

func TestIntegration(t *testing.T) {
	s := Secret{
		Secret: "I'm a secret!",
		Attachments: []SecretAttachment{{
			Name:    "secret.txt",
			Type:    "text/plain",
			Content: []byte("I'm a very secret file.\n"),
		}},
	}

	var storedSecret []byte
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/create" {
			var payload struct {
				Secret string `json:"secret"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			storedSecret = []byte(payload.Secret)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"secret_id":"local_sec_100"}`))
			return
		}
		if r.URL.Path == "/api/get/local_sec_100" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"secret":%q}`, string(storedSecret))))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	secretURL, _, err := Create(testServer.URL, s, time.Minute)
	require.NoError(t, err)
	assert.Contains(t, secretURL, "#local_sec_100%7C")

	apiSecret, err := Fetch(secretURL)
	require.NoError(t, err)

	assert.Equal(t, s, apiSecret)
}

func TestCreateErrorHandlingAndExpireQuery(t *testing.T) {
	// Test Create with custom expireIn parameter
	s := Secret{Secret: "test_secret"}

	var queryExpire string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryExpire = r.URL.Query().Get("expire")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"secret_id":"12345"}`))
	}))
	defer testServer.Close()

	secretURL, _, err := Create(testServer.URL, s, 3600*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "3600", queryExpire)
	assert.Contains(t, secretURL, "#12345%7C")

	// Test server returning HTTP 400 Bad Request
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer errServer.Close()

	_, _, err = Create(errServer.URL, s, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http error: status 400")

	// Test server returning invalid JSON
	invalidJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`not_json`))
	}))
	defer invalidJSONServer.Close()

	_, _, err = Create(invalidJSONServer.URL, s, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding response")
}

func TestFetchErrorHandling(t *testing.T) {
	// Test Fetch returning HTTP 404 Not Found
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer errServer.Close()

	_, err := Fetch(errServer.URL + "/#dummyid|dummypass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 404")

	// Test Fetch returning invalid JSON
	invalidJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid_json`))
	}))
	defer invalidJSONServer.Close()

	_, err = Fetch(invalidJSONServer.URL + "/#dummyid|dummypass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding response body")
}

func TestLoadSettingsErrorHandling(t *testing.T) {
	// Test LoadSettings with invalid URL
	_, err := LoadSettings(":%invalid_url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing instance URL")

	// Test LoadSettings returning 404 Not Found
	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFoundServer.Close()

	_, err = LoadSettings(notFoundServer.URL)
	assert.ErrorIs(t, err, errSettingsNotFound)

	// Test LoadSettings returning invalid JSON
	invalidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid_json`))
	}))
	defer invalidServer.Close()

	_, err = LoadSettings(invalidServer.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding response")
}

func TestCLICreateRawWithOpts(t *testing.T) {
	var receivedRawBody []byte
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/create/raw" {
			var err error
			receivedRawBody, err = io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"secret_id":"raw_secret_999"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	s := Secret{Secret: "CLI Raw Binary Secret Message"}
	secretURL, _, err := CreateRawWithOpts(testServer.URL, s, CreateOpts{Reads: 1})
	require.NoError(t, err)
	assert.Contains(t, secretURL, "#raw_secret_999")
	assert.NotEmpty(t, receivedRawBody)
}

func TestBurn(t *testing.T) {
	var requestedPath string
	var requestedMethod string

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedMethod = r.Method
		if r.URL.Path == "/api/burn/target_secret_123" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"reads_remaining":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	// 1. Test Burn with unified secret URL
	secretURL := testServer.URL + "/#target_secret_123%7Cpassword_abc"
	err := Burn(secretURL)
	require.NoError(t, err)
	assert.Equal(t, "/api/burn/target_secret_123", requestedPath)
	assert.Equal(t, http.MethodPost, requestedMethod)

	// 2. Test Burn with base short URL (Channel 1)
	shortURL := testServer.URL + "/#target_secret_123"
	err = Burn(shortURL)
	require.NoError(t, err)
	assert.Equal(t, "/api/burn/target_secret_123", requestedPath)
	assert.Equal(t, http.MethodPost, requestedMethod)

	// 3. Test Burn returning HTTP 404
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer errServer.Close()

	err = Burn(errServer.URL + "/#missing_secret%7Cpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 404")
}

func TestSplitSecretURLAndFetchWithKey(t *testing.T) {
	// Test SplitSecretURL valid
	unifiedURL := "http://localhost:3000/#secret123%7Cpass456"
	baseURL, key, err := SplitSecretURL(unifiedURL)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:3000/#secret123", baseURL)
	assert.Equal(t, "pass456", key)

	// Test SplitSecretURL invalid fragment format
	_, _, err = SplitSecretURL("http://localhost:3000/#no_pipe_key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing decryption key fragment")

	// Test SplitSecretURL invalid URL
	_, _, err = SplitSecretURL(":%invalid_url")
	assert.Error(t, err)

	// Test FetchWithKey missing key
	_, err = FetchWithKey("http://localhost:3000/#secret123", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption key missing")

	// Test Burn invalid URL or missing secret ID
	err = Burn(":%invalid_url")
	assert.Error(t, err)

	err = Burn("http://localhost:3000/#")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing secret ID")

	// Edge-case URL parsing errors
	_, _, err = CreateWithOpts(":%invalid_url", Secret{}, CreateOpts{})
	assert.Error(t, err)

	_, _, err = CreateRawWithOpts(":%invalid_url", Secret{}, CreateOpts{})
	assert.Error(t, err)

	_, err = FetchWithKey(":%invalid_url", "key")
	assert.Error(t, err)

	_, err = FetchWithKey("http://localhost/#invalid%XX", "")
	assert.Error(t, err)

	err = Burn("http://localhost/#invalid%XX")
	assert.Error(t, err)
}

func TestLoadSettingsSuccessAndRequestFailure(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"appTitle":"Test Instance"}`))
	}))
	defer s.Close()

	cust, err := LoadSettings(s.URL)
	require.NoError(t, err)
	assert.Equal(t, "Test Instance", cust.AppTitle)

	_, err = LoadSettings("http://127.0.0.1:0/unreachable")
	assert.Error(t, err)
}
