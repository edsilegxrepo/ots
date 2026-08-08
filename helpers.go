package main

import (
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/edsilegxrepo/ots/pkg/metrics"
	"github.com/edsilegxrepo/ots/pkg/storage"
)

func requestInSubnetList(r *http.Request, subnets []string) bool {
	if len(subnets) == 0 {
		// No subnets specififed: None allowed (without doing the parsing)
		return false
	}

	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logrus.WithError(err).Error("parsing remote address")
		return false
	}

	remoteIP := net.ParseIP(remote)
	if remoteIP == nil {
		logrus.WithError(err).Error("parsing remote address")
		return false
	}

	for _, sn := range subnets {
		_, netw, err := net.ParseCIDR(sn)
		if err != nil {
			logrus.WithError(err).WithField("subnet", sn).Warn("invalid subnet specified")
			continue
		}

		if netw.Contains(remoteIP) {
			return true
		}
	}

	return false
}

func updateStoredSecretsCount(store storage.Storage, collector *metrics.Collector) {
	n, err := store.Count()
	if err != nil {
		logrus.WithError(err).Error("counting stored secrets")
		return
	}
	collector.UpdateSecretsCount(n)
}

// fsStack provides a layered fs.ReadFileFS implementation over multiple fs.FS instances.
type fsStack []fs.FS

func (f fsStack) Open(name string) (fs.File, error) {
	for _, fsys := range f {
		if file, err := fsys.Open(name); err == nil {
			return file, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (f fsStack) ReadFile(name string) ([]byte, error) {
	for _, fsys := range f {
		if rfs, ok := fsys.(fs.ReadFileFS); ok {
			if data, err := rfs.ReadFile(name); err == nil {
				return data, nil
			}
		} else if file, err := fsys.Open(name); err == nil {
			data, err := io.ReadAll(file)
			_ = file.Close()
			if err == nil {
				return data, nil
			}
		}
	}
	return nil, fs.ErrNotExist
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	// Skip gzip encoding for 204 No Content, 304 Not Modified, or 1xx statuses
	if code == http.StatusNoContent || code == http.StatusNotModified || (code >= 100 && code < 200) {
		w.Header().Del("Content-Encoding")
		w.ResponseWriter.WriteHeader(code)
		return
	}

	w.Header().Set("Content-Encoding", "gzip")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.Header().Get("Content-Encoding") == "gzip" {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzip.NewWriter(w)
		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			gz:             gz,
		}
		defer func() {
			if gzw.Header().Get("Content-Encoding") == "gzip" {
				_ = gz.Close()
			}
		}()

		next.ServeHTTP(gzw, r)
	})
}

func httpLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		logrus.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"remote":     r.RemoteAddr,
			"status":     rw.statusCode,
			"duration":   time.Since(start).String(),
			"user_agent": r.UserAgent(),
		}).Info("HTTP Request")
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CSP helps build Content-Security-Policy header strings.
type CSP map[string][]string

func (c CSP) Add(directive, src string) {
	c[directive] = append(c[directive], src)
}

func (c CSP) String() string {
	var parts []string
	for k, v := range c {
		parts = append(parts, fmt.Sprintf("%s %s", k, strings.Join(v, " ")))
	}
	return strings.Join(parts, "; ")
}

const (
	CSPSrcSelf       = "'self'"
	CSPSrcNone       = "'none'"
	CSPSrcSchemeData = "data:"
)

func CSPSrcNonce(nonce string) string {
	return fmt.Sprintf("'nonce-%s'", nonce)
}

func (c CSP) ToHeaderValue() string {
	return c.String()
}

var byteBufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, 64*1024) // 64KB buffer pool
		return &b
	},
}

// DecodeBase64Pooled decodes input Base64 using a zero-allocation sync.Pool buffer
func DecodeBase64Pooled(s string) ([]byte, error) {
	bufPtr := byteBufferPool.Get().(*[]byte)
	defer byteBufferPool.Put(bufPtr)

	buf := *bufPtr
	enc := base64.StdEncoding
	if strings.Contains(s, "-") || strings.Contains(s, "_") {
		enc = base64.RawURLEncoding
	}

	decodedLen := enc.DecodedLen(len(s))
	if decodedLen > len(buf) {
		return enc.DecodeString(s)
	}

	n, err := enc.Decode(buf[:decodedLen], []byte(s))
	if err != nil {
		return base64.URLEncoding.DecodeString(s)
	}

	res := make([]byte, n)
	copy(res, buf[:n])
	return res, nil
}
