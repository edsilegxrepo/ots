package client

import (
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

	secretURL, _, err := Create("https://ots.fyi/", s, time.Minute)
	require.NoError(t, err)
	assert.Regexp(t, `^https://ots.fyi/#[0-9a-f-]+%7C[0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ]+$`, secretURL)

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
