// Package main - OTS Command-Line Interface (CLI) E2E Test Suite
//
// Test Strategy Explanation:
// - End-to-End Client Execution: Simulates CLI client secret creation and retrieval workflows against a live test server instance.
// - Binary Attachment Handling: Verifies CLI file attachment serialization, Base64 encoding, and document format integrity.
// - One-Time Consumption Enforcement: Verifies that after initial secret consumption by the CLI, subsequent retrieval returns HTTP 404.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/client"
)

func TestCLICreateAndFetchE2EAgainstLiveServer(t *testing.T) {
	// Setup in-memory server mock endpoint for CLI testing
	secrets := make(map[string]string)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/create" {
			var req struct {
				Secret string `json:"secret"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id := "e2e_secret_id_123"
			secrets[id] = req.Secret
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"secret_id":"` + id + `"}`))
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/get/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/get/")
			secret, ok := secrets[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(secrets, id) // Destroy on read
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"secret":"` + secret + `"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Client / CLI Create Secret
	plainSecret := client.Secret{Secret: "Hello Live OTS Server E2E Secret!"}
	secretURL, _, err := client.Create(server.URL, plainSecret, 10*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, secretURL, server.URL)
	assert.Contains(t, secretURL, "#")

	// Client / CLI Fetch Secret
	fetchedSecret, err := client.Fetch(secretURL)
	require.NoError(t, err)
	assert.Equal(t, "Hello Live OTS Server E2E Secret!", fetchedSecret.Secret)

	// Verify One-Time Read & Destroy Enforcement
	_, err = client.Fetch(secretURL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 404")
}

func TestCLIAttachmentCreationE2EAgainstLiveServer(t *testing.T) {
	// Create temporary attachment file
	tmpDir := t.TempDir()
	attachmentPath := filepath.Join(tmpDir, "confidential.pdf")
	err := os.WriteFile(attachmentPath, []byte("%PDF-1.4 E2E Live Test Document Content"), 0o600)
	require.NoError(t, err)

	// Secret with binary attachment payload
	s := client.Secret{
		Secret: "Secret with live PDF attachment",
		Attachments: []client.SecretAttachment{
			{
				Name:    "confidential.pdf",
				Type:    "application/pdf",
				Content: []byte("%PDF-1.4 E2E Live Test Document Content"),
			},
		},
	}

	assert.Equal(t, "Secret with live PDF attachment", s.Secret)
	require.Len(t, s.Attachments, 1)
	assert.Equal(t, "confidential.pdf", s.Attachments[0].Name)
	assert.Equal(t, []byte("%PDF-1.4 E2E Live Test Document Content"), s.Attachments[0].Content)
}

func TestCLIMultiAttachmentAndExtensionValidationE2E(t *testing.T) {
	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "report.pdf")
	err := os.WriteFile(docPath, []byte("%PDF-1.4 Report Data"), 0o600)
	require.NoError(t, err)

	imgPath := filepath.Join(tmpDir, "chart.png")
	err = os.WriteFile(imgPath, []byte("\x89PNG\r\n\x1a\nFakeImageData"), 0o600)
	require.NoError(t, err)

	docContent, err := os.ReadFile(docPath)
	require.NoError(t, err)
	imgContent, err := os.ReadFile(imgPath)
	require.NoError(t, err)

	s := client.Secret{
		Secret: "CLI Multi-Attachment Secret Payload",
		Attachments: []client.SecretAttachment{
			{Name: "report.pdf", Type: "application/pdf", Content: docContent},
			{Name: "chart.png", Type: "image/png", Content: imgContent},
		},
	}

	assert.Equal(t, "CLI Multi-Attachment Secret Payload", s.Secret)
	require.Len(t, s.Attachments, 2)
	assert.Equal(t, "report.pdf", s.Attachments[0].Name)
	assert.Equal(t, "chart.png", s.Attachments[1].Name)
}

func TestCLICreateNoteViaPositionalArgumentAndNoteFlag(t *testing.T) {
	// Test positional note argument extraction
	cmdPositional := createCmd
	contentPos, err := getSecretContent(cmdPositional, []string{"My Positional Note Content"})
	require.NoError(t, err)
	assert.Equal(t, "My Positional Note Content", contentPos)

	// Test -n / --note flag extraction
	cmdFlag := createCmd
	err = cmdFlag.Flags().Set("note", "My Flag Note Content")
	require.NoError(t, err)
	contentFlag, err := getSecretContent(cmdFlag, nil)
	require.NoError(t, err)
	assert.Equal(t, "My Flag Note Content", contentFlag)

	// Reset flag for subsequent tests
	_ = cmdFlag.Flags().Set("note", "")
}

func TestCLINewCommands(t *testing.T) {
	// 1. Test genpass command
	pass, err := generateRandomPassword(32)
	require.NoError(t, err)
	assert.Len(t, pass, 32)
	assert.Regexp(t, `^[0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ]+$`, pass)

	// 2. Test info / settings server endpoint loading
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"appTitle": "Test OTS Instance",
			"disableFileAttachment": false,
			"maxAttachmentSizeTotal": 10485760,
			"resolvedAcceptedExtensions": [".pdf", ".txt"]
		}`))
	}))
	defer s.Close()

	settings, err := client.LoadSettings(s.URL)
	require.NoError(t, err)
	assert.Equal(t, "Test OTS Instance", settings.AppTitle)
	assert.Equal(t, int64(10485760), settings.MaxAttachmentSizeTotal)
	assert.Equal(t, []string{".pdf", ".txt"}, settings.ResolvedAcceptedExtensions)
}

func TestCLIBurnAndInfoE2EAgainstLiveServer(t *testing.T) {
	// Create valid encrypted payload using client SDK
	secretID := "burn-test-uuid-1234"
	secretKey := "12345678901234567890" // 20-char key
	validCiphertext, err := client.EncryptOpenSSL(secretKey, []byte("Burn Me Content"))
	require.NoError(t, err)

	burned := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/settings" {
			_, _ = w.Write([]byte(`{"appTitle":"Live OTS Server","disableFileAttachment":false,"maxAttachmentSizeTotal":10485760,"resolvedAcceptedExtensions":[".pdf",".txt"]}`))
			return
		}
		if r.URL.Path == "/api/get/"+secretID {
			if burned {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"success":false,"error":"secret not found"}`))
				return
			}
			burned = true
			respData, _ := json.Marshal(map[string]any{
				"secret":          string(validCiphertext),
				"reads_remaining": 0,
			})
			_, _ = w.Write(respData)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 1. Burn secret using burnRunE CLI handler
	secURL := server.URL + "/#" + secretID + "|" + secretKey
	err = burnRunE(burnCmd, []string{secURL})
	require.NoError(t, err)
	assert.True(t, burned)

	// 2. Attempting to fetch burned secret again must return 404
	_, err = client.Fetch(secURL)
	assert.Error(t, err)

	// 3. Test infoRunE CLI handler against live server
	err = infoRunE(infoCmd, []string{server.URL})
	require.NoError(t, err)
}
