package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestInSubnetListHelper(t *testing.T) {
	// 1. Empty subnets list returns false
	req1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	assert.False(t, requestInSubnetList(req1, nil))

	// 2. Matching IPv4 CIDR
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	req2.RemoteAddr = "10.0.4.5:54321"
	assert.True(t, requestInSubnetList(req2, []string{"10.0.0.0/8", "127.0.0.1/32"}))

	// 3. Non-matching IP
	req3 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	req3.RemoteAddr = "192.168.1.50:54321"
	assert.False(t, requestInSubnetList(req3, []string{"10.0.0.0/8", "127.0.0.1/32"}))

	// 4. Invalid RemoteAddr host/port format
	req4 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	req4.RemoteAddr = "invalid_remote_addr"
	assert.False(t, requestInSubnetList(req4, []string{"10.0.0.0/8"}))
}

func TestDecodeBase64Pooled(t *testing.T) {
	// 1. Standard Base64
	srcStd := base64.StdEncoding.EncodeToString([]byte("hello standard base64"))
	decodedStd, err := DecodeBase64Pooled(srcStd)
	require.NoError(t, err)
	assert.Equal(t, "hello standard base64", string(decodedStd))

	// 2. URL-Safe Base64
	srcURL := base64.RawURLEncoding.EncodeToString([]byte("hello_url-safe_base64"))
	decodedURL, err := DecodeBase64Pooled(srcURL)
	require.NoError(t, err)
	assert.Equal(t, "hello_url-safe_base64", string(decodedURL))

	// 3. Large payload exceeding pooled 64KB buffer
	largeData := bytes.Repeat([]byte("A"), 70*1024)
	srcLarge := base64.StdEncoding.EncodeToString(largeData)
	decodedLarge, err := DecodeBase64Pooled(srcLarge)
	require.NoError(t, err)
	assert.Equal(t, largeData, decodedLarge)
}

func TestGenerateUUID(t *testing.T) {
	uuid1 := GenerateUUID()
	uuid2 := GenerateUUID()

	assert.Len(t, uuid1, 36)
	assert.Len(t, uuid2, 36)
	assert.NotEqual(t, uuid1, uuid2)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, uuid1)
}

func TestFSStack(t *testing.T) {
	fs1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content of file 1")},
	}
	fs2 := fstest.MapFS{
		"file2.txt": &fstest.MapFile{Data: []byte("content of file 2")},
	}

	stack := fsStack{fs1, fs2}

	// Read file1 from fs1
	data1, err := stack.ReadFile("file1.txt")
	require.NoError(t, err)
	assert.Equal(t, "content of file 1", string(data1))

	// Read file2 from fs2
	data2, err := stack.ReadFile("file2.txt")
	require.NoError(t, err)
	assert.Equal(t, "content of file 2", string(data2))

	// Read non-existent file
	_, err = stack.ReadFile("missing.txt")
	assert.ErrorIs(t, err, fs.ErrNotExist)

	// Open file1
	f1, err := stack.Open("file1.txt")
	require.NoError(t, err)
	_ = f1.Close()
}

func TestGzipMiddleware(t *testing.T) {
	handler := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("gzipped response content payload"))
	}))

	// 1. Without Accept-Encoding: gzip
	req1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/data", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Empty(t, w1.Header().Get("Content-Encoding"))
	assert.Equal(t, "gzipped response content payload", w1.Body.String())

	// 2. With Accept-Encoding: gzip
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/data", nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, "gzip", w2.Header().Get("Content-Encoding"))

	gr, err := gzip.NewReader(w2.Body)
	require.NoError(t, err)
	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, "gzipped response content payload", string(decompressed))
}

func TestGzipNoContentHeaderExclusion(t *testing.T) {
	handler := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nocontent", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Header().Get("Content-Encoding"))
}

func TestCSPBuilder(t *testing.T) {
	csp := make(CSP)
	csp.Add("default-src", CSPSrcSelf)
	csp.Add("script-src", CSPSrcNonce("testnonce123"))

	assert.Equal(t, "'nonce-testnonce123'", CSPSrcNonce("testnonce123"))
	headerVal := csp.ToHeaderValue()
	assert.Contains(t, headerVal, "default-src 'self'")
	assert.Contains(t, headerVal, "script-src 'nonce-testnonce123'")
}
