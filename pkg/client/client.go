// Package client implements a client library for OTS supporting the
// OTSMeta content format for file upload support.
//
// Objectives:
// - Implements zero-knowledge client-side encryption and decryption of secrets and file attachments.
// - Provides a Go SDK for creating, fetching, and dual-channel splitting of OTS secret URLs.
// - Ensures cross-interoperability between the Go CLI, Go SDK, and Vue SPA browser frontend.
//
// Core Components:
// - Create: Generates a cryptographically secure key, encrypts secret & attachments via PBKDF2/AES, and posts to server.
// - Fetch: Retrieves encrypted secret blob from server and decrypts payload using URL fragment key.
// - FetchWithKey: Supports dual-channel decryption when secret URL and key are transmitted separately (Issue #208).
// - SplitSecretURL: Utility function splitting unified URL into base URL (#secret_id) and decryption key.
//
// Data Flow:
// 1. Secret Payload -> PBKDF2/OpenSSL AES Encryption -> JSON Payload -> POST /api/create -> Server returns Secret ID.
// 2. Secret URL -> GET /api/get/{secret_id} -> Server returns Encrypted Blob -> Client Decrypts Payload via Key -> Secret Restored.
package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type (
	// HTTPClientIntf describes a minimal interface to be fulfilled
	// by the given HTTP client. This can be used for mocking and to
	// pass in authenticated clients
	HTTPClientIntf interface {
		// Do is the expected method on the HTTP client to do the request
		Do(*http.Request) (*http.Response, error)
	}
)

// HTTPClient defines the client to use for create and fetch requests
// and can be overwritten to provide authentication
var HTTPClient HTTPClientIntf = http.DefaultClient

// Logger can be set to enable logging from the library. By default
// all log-messages will be discarded.
var Logger *logrus.Entry

// PasswordLength defines the length of the generated encryption password
var PasswordLength = 32

// RequestTimeout defines how long the request to the OTS instance for
// create and fetch may take
var RequestTimeout = 5 * time.Second

// UserAgent defines the user-agent to send when interacting with an
// OTS instance. When using this library please set this to something
// the operator of the instance can determine your client from and
// provide an URL to useful information about your tool.
var UserAgent = "ots-client/1.x +https://github.com/Luzifer/ots"

func init() {
	l := logrus.New()
	l.SetOutput(io.Discard)
	Logger = logrus.NewEntry(l)
}

// CreateOpts specifies configurable parameters for secret creation in the Go client SDK.
type CreateOpts struct {
	ExpireIn time.Duration
	Reads    int
}

// Create serializes the secret and creates a new secret on the
// instance given by its URL.
//
// The given URL should point to the frontend of the instance. Do not
// include the API paths, they are added automatically. For the
// expireIn parameter zero value can be used to use server-default.
//
// So for OTS.fyi you'd use `New("https://ots.fyi/")`
func Create(instanceURL string, secret Secret, expireIn time.Duration) (string, time.Time, error) {
	return CreateWithOpts(instanceURL, secret, CreateOpts{
		ExpireIn: expireIn,
		Reads:    1,
	})
}

// CreateWithOpts serializes the secret and creates a new secret on the instance with customizable options.
func CreateWithOpts(instanceURL string, secret Secret, opts CreateOpts) (string, time.Time, error) {
	u, err := url.Parse(instanceURL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parsing instance URL: %w", err)
	}

	pass, err := genPass()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generating password: %w", err)
	}

	data, err := secret.serialize(pass)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("serializing data: %w", err)
	}

	bodyPayload := struct {
		Reads  int    `json:"reads,omitempty"`
		Secret string `json:"secret"` //#nosec:G117 // This application works with secrets
	}{
		Reads:  opts.Reads,
		Secret: string(data),
	}

	body := new(bytes.Buffer)
	if err = json.NewEncoder(body).Encode(bodyPayload); err != nil {
		return "", time.Time{}, fmt.Errorf("encoding request payload: %w", err)
	}

	createURL := u.JoinPath(strings.Join([]string{".", "api", "create"}, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	if opts.ExpireIn > time.Second {
		createURL.RawQuery = url.Values{
			"expire": []string{strconv.Itoa(int(opts.ExpireIn / time.Second))},
		}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL.String(), body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // possible leaked-fd, lib should not log, potential short-lived leak

	if resp.StatusCode != http.StatusCreated {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("http error: status %d", resp.StatusCode)
		}
		return "", time.Time{}, fmt.Errorf("http error: status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		ExpiresAt *time.Time `json:"expires_at"`
		SecretID  string     `json:"secret_id"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", time.Time{}, fmt.Errorf("decoding response payload: %w", err)
	}

	u.Fragment = strings.Join([]string{res.SecretID, pass}, "|")

	var expiresAt time.Time
	if res.ExpiresAt != nil {
		expiresAt = *res.ExpiresAt
	}

	return u.String(), expiresAt, nil
}

// CreateRawWithOpts posts the encrypted binary payload directly to /api/create/raw using application/octet-stream,
// eliminating client and server Base64 and JSON encoding overhead.
func CreateRawWithOpts(instanceURL string, secret Secret, opts CreateOpts) (string, time.Time, error) {
	u, err := url.Parse(instanceURL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parsing instance URL: %w", err)
	}

	pass, err := genPass()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generating password: %w", err)
	}

	data, err := secret.serialize(pass)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("serializing data: %w", err)
	}

	createURL := u.JoinPath(strings.Join([]string{".", "api", "create", "raw"}, "/"))
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	if opts.ExpireIn > time.Second {
		createURL.RawQuery = url.Values{
			"expire": []string{strconv.Itoa(int(opts.ExpireIn / time.Second))},
		}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL.String(), bytes.NewReader(data))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		// Fallback to standard CreateWithOpts if server does not support /api/create/raw (e.g. 404 Not Found)
		if resp.StatusCode == http.StatusNotFound {
			return CreateWithOpts(instanceURL, secret, opts)
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("http error: status %d", resp.StatusCode)
		}
		return "", time.Time{}, fmt.Errorf("http error: status %d: %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		ExpiresAt *time.Time `json:"expires_at"`
		SecretID  string     `json:"secret_id"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", time.Time{}, fmt.Errorf("decoding response payload: %w", err)
	}

	u.Fragment = strings.Join([]string{res.SecretID, pass}, "|")

	var expiresAt time.Time
	if res.ExpiresAt != nil {
		expiresAt = *res.ExpiresAt
	}

	return u.String(), expiresAt, nil
}

// Fetch retrieves a secret by its given URL. The URL given must
// include the fragment (part after the `#`) with the secret ID and
// the encryption passphrase.
//
// The object returned will always be an OTSMeta object even in case
// the secret is a plain secret without attachments.
func Fetch(secretURL string) (s Secret, err error) {
	return FetchWithKey(secretURL, "")
}

// FetchWithKey retrieves a secret when the secret URL and decryption key
// are split for separate channel transmission (Dual-Channel Delivery).
func FetchWithKey(secretURL, decryptionKey string) (s Secret, err error) {
	u, err := url.Parse(secretURL)
	if err != nil {
		return s, fmt.Errorf("parsing secret URL: %w", err)
	}

	fragment, err := url.QueryUnescape(u.Fragment)
	if err != nil {
		return s, fmt.Errorf("unescaping fragment: %w", err)
	}
	fragmentParts := strings.SplitN(fragment, "|", 2)

	key := decryptionKey
	if len(fragmentParts) > 1 && fragmentParts[1] != "" {
		key = fragmentParts[1]
	}

	if key == "" {
		return s, fmt.Errorf("decryption key missing from URL fragment and parameter")
	}

	fetchURL := u.JoinPath(strings.Join([]string{".", "api", "get", fragmentParts[0]}, "/")).String()
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return s, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return s, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // possible leaked-fd, lib should not log, potential short-lived leak

	if resp.StatusCode != http.StatusOK {
		return s, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	var payload struct {
		Secret string `json:"secret"` //#nosec:G117 // This application works with secrets
	}

	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return s, fmt.Errorf("decoding response body: %w", err)
	}

	if err = s.read([]byte(payload.Secret), key); err != nil {
		return s, fmt.Errorf("decoding secret: %w", err)
	}

	return s, nil
}

// SplitSecretURL splits a unified secret URL into a base URL (Channel A)
// and a separate decryption key (Channel B) for dual-channel transmission.
func SplitSecretURL(secretURL string) (baseURL, decryptionKey string, err error) {
	u, err := url.Parse(secretURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing secret URL: %w", err)
	}

	fragment, err := url.QueryUnescape(u.Fragment)
	if err != nil {
		return "", "", fmt.Errorf("unescaping fragment: %w", err)
	}

	parts := strings.SplitN(fragment, "|", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid secret URL format: missing decryption key fragment")
	}

	u.Fragment = parts[0]
	return u.String(), parts[1], nil
}

func genPass() (string, error) {
	var (
		charSet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		pass    = make([]byte, PasswordLength)

		n   int
		err error
	)

	for n < PasswordLength {
		n, err = rand.Read(pass)
		if err != nil {
			return "", fmt.Errorf("reading random data: %w", err)
		}
	}

	for i := 0; i < PasswordLength; i++ {
		pass[i] = charSet[int(pass[i])%len(charSet)]
	}

	return string(pass), nil
}
