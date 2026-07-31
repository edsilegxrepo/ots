package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	filehelpers "github.com/Luzifer/go_helpers/file"
	httphelpers "github.com/Luzifer/go_helpers/http"
	"github.com/Luzifer/rconfig/v2"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/Luzifer/ots/pkg/customization"
	"github.com/Luzifer/ots/pkg/metrics"
)

const scriptNonceSize = 32

var (
	cfg struct {
		Customize      string `flag:"customize" default:"" description:"Customize-File to load"`
		Listen         string `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		LogRequests    bool   `flag:"log-requests" default:"true" description:"Enable request logging"`
		LogLevel       string `flag:"log-level" default:"info" description:"Set log level (debug, info, warning, error)"`
		LogFilePath    string `flag:"log-file-path" default:"" description:"Path to file for appending log output (e.g. /var/log/ots.log)"`
		LogFormat      string `flag:"log-format" default:"text" description:"Set log output format (text, json, ndjson)"`
		SecretExpiry   int64  `flag:"secret-expiry" default:"86400" description:"Maximum expiry of the stored secrets in seconds"`
		StorageType    string `flag:"storage-type" default:"mem" description:"Storage to use for putting secrets to" validate:"nonzero"` //revive:disable-line:struct-tag // Matches wrong validation library
		VersionAndExit bool   `flag:"version" default:"false" description:"Print version information and exit"`
		EnableTLS      bool   `flag:"enable-tls" default:"false" description:"Enable HTTPS/TLS"`
		CertFile       string `flag:"cert-file" default:"" description:"Path to the TLS certificate file"`
		KeyFile        string `flag:"key-file" default:"" description:"Path to the TLS private key file"`
	}

	assets   filehelpers.FSStack
	cust     customization.Customize
	indexTpl *template.Template

	version = "dev"
)

//go:embed frontend/*
var embeddedAssets embed.FS

func defaultCSP() httphelpers.CSP {
	c := httphelpers.CSP{}

	c.Add("base-uri", httphelpers.CSPSrcSelf)
	c.Add("default-src", httphelpers.CSPSrcNone)
	c.Add("connect-src", httphelpers.CSPSrcSelf)
	c.Add("font-src", httphelpers.CSPSrcSelf)
	c.Add("img-src", httphelpers.CSPSrcSelf)
	c.Add("img-src", httphelpers.CSPSrcSchemeData)
	c.Add("script-src", httphelpers.CSPSrcSelf)
	c.Add("style-src", httphelpers.CSPSrcSelf)

	return c
}

func hardenListener(listen string, enableTLS bool) string {
	if !enableTLS {
		if listen != ":3000" {
			// User explicitly specified custom --listen (e.g. for Docker, K8s, or external reverse proxy)
			host, _, err := net.SplitHostPort(listen)
			if err == nil && (host == "" || host == "0.0.0.0" || host == "::") {
				logrus.WithField("listen", listen).Warn("TLS is disabled while listening on a wildcard interface. Ensure an upstream reverse proxy terminates HTTPS.")
			}
			return listen
		}
		// Unset/default listener: harden to loopback 127.0.0.1:3000 by default
		logrus.WithField("listen", "127.0.0.1:3000").Info("TLS not enabled: hardening default HTTP listener to loopback 127.0.0.1:3000")
		return "127.0.0.1:3000"
	}
	return listen
}

func initApp() error {
	rconfig.AutoEnv(true)
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		return fmt.Errorf("parsing cli options: %w", err)
	}

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("parsing log-level: %w", err)
	}
	logrus.SetLevel(l)

	switch strings.ToLower(cfg.LogFormat) {
	case "json", "ndjson":
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	default:
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	if cfg.LogFilePath != "" {
		logFile, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening log-file-path %q: %w", cfg.LogFilePath, err)
		}
		logrus.SetOutput(logFile)
	}

	if cust, err = customization.Load(cfg.Customize); err != nil {
		return fmt.Errorf("loading customizations: %w", err)
	}

	frontendFS, err := fs.Sub(embeddedAssets, "frontend")
	if err != nil {
		return fmt.Errorf("creating sub-fs for assets: %W", err)
	}
	assets = append(assets, frontendFS)

	if cust.OverlayFSPath != "" {
		assets = append(filehelpers.FSStack{os.DirFS(cust.OverlayFSPath)}, assets...)
	}

	cfg.Listen = hardenListener(cfg.Listen, cfg.EnableTLS)

	return nil
}

const (
	ExitSuccess      = 0
	ExitConfigError  = 2
	ExitStorageError = 3
	ExitNetworkError = 4
)

func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Error("initializing app")
		os.Exit(ExitConfigError)
	}

	if cfg.VersionAndExit {
		logrus.WithField("version", version).Info("ots")
		os.Exit(ExitSuccess)
	}

	// Initialize metrics collector
	collector := metrics.New()

	// Initialize index template in order not to parse it multiple times
	source, err := assets.ReadFile("index.html")
	if err != nil {
		logrus.WithError(err).Error("frontend folder should contain index.html Go template")
		os.Exit(ExitConfigError)
	}
	indexTpl = template.Must(template.New("index.html").Funcs(tplFuncs).Parse(string(source)))

	// Initialize storage
	store, err := getStorageByType(cfg.StorageType)
	if err != nil {
		logrus.WithError(err).Error("initializing storage")
		os.Exit(ExitStorageError)
	}
	api := NewAPI(store, collector)

	// Initialize server
	r := mux.NewRouter()

	api.Register(r.PathPrefix("/api").Subrouter())

	r.Handle("/metrics", handleRemoveAcceptEncoding(metrics.Handler())).
		Methods(http.MethodGet).
		MatcherFunc(func(r *http.Request, _ *mux.RouteMatch) bool {
			return requestInSubnetList(r, cust.MetricsAllowedSubnets)
		})

	r.HandleFunc("/robots.txt", handleRobots).
		Methods(http.MethodGet)
	r.HandleFunc("/", handleIndex).
		Methods(http.MethodGet)
	r.PathPrefix("/").HandlerFunc(assetDelivery).
		Methods(http.MethodGet)

	var hdl http.Handler = r
	hdl = httphelpers.GzipHandler(hdl)
	if cfg.LogRequests {
		hdl = httphelpers.NewHTTPLogHandlerWithLogger(hdl, logrus.StandardLogger())
	}

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           hdl,
		ReadHeaderTimeout: time.Second,
	}

	// Start periodic stored metrics update (required for multi-instance
	// OTS hosting as other instances will create / delete secrets and
	// we need to keep up with that)
	go func() {
		for t := time.NewTicker(time.Minute); ; <-t.C {
			updateStoredSecretsCount(store, collector)
		}
	}()

	if cfg.EnableTLS && (cfg.CertFile == "" || cfg.KeyFile == "") {
		logrus.Error("TLS is enabled but cert-file or key-file is not provided")
		os.Exit(ExitConfigError)
	}

	// Channel to listen for signal interrupt / termination
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logrus.WithFields(logrus.Fields{
			"secret_expiry": time.Duration(cfg.SecretExpiry) * time.Second,
			"version":       version,
		}).Info("ots started")

		var serveErr error
		if cfg.EnableTLS {
			logrus.Infof("Starting HTTPS server on %s", cfg.Listen)
			serveErr = server.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
		} else {
			logrus.Infof("Starting HTTP server on %s", cfg.Listen)
			serveErr = server.ListenAndServe()
		}

		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logrus.WithError(serveErr).Error("HTTP server quit unexpectedly")
			os.Exit(ExitNetworkError)
		}
	}()

	<-stop
	logrus.Info("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		logrus.WithError(shutdownErr).Error("Server forced to shutdown")
		os.Exit(ExitNetworkError)
	}

	logrus.Info("Server stopped cleanly")
	os.Exit(ExitSuccess)
}

func assetDelivery(w http.ResponseWriter, r *http.Request) {
	assetName := strings.TrimLeft(r.URL.Path, "/")

	dot := strings.LastIndex(assetName, ".")
	if dot < 0 {
		// There are no assets with no dot in it
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

	ext := assetName[dot:]
	assetData, err := assets.ReadFile(assetName)
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(ext))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Safe: Serving static binary/text frontend asset bytes read from embedded filesystem
	// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
	if _, err = w.Write(assetData); err != nil { //#nosec:G705 // False positive
		logrus.WithError(err).Error("writing asset data")
	}
}

func handleRobots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if cust.IsSearchIndexDisabled() {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")
		_, _ = fmt.Fprintln(w, "User-agent: *")
		_, _ = fmt.Fprintln(w, "Disallow: /")
		return
	}

	_, _ = fmt.Fprintln(w, "User-agent: *")
	_, _ = fmt.Fprintln(w, "Allow: /")
}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	inlineContentNonce := make([]byte, scriptNonceSize)
	if _, err := rand.Read(inlineContentNonce); err != nil {
		logrus.WithError(err).Error("generating script nonce")
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	inlineContentNonceStr := base64.StdEncoding.EncodeToString(inlineContentNonce)

	policy := defaultCSP()
	policy.Add("script-src", httphelpers.CSPSrcNonce(inlineContentNonceStr))
	policy.Add("style-src", httphelpers.CSPSrcNonce(inlineContentNonceStr))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Xss-Protection", "1; mode=block")
	w.Header().Set("Content-Security-Policy", policy.ToHeaderValue())
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if cust.IsSearchIndexDisabled() {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")
	}

	if err := indexTpl.Execute(w, struct {
		Customize          customization.Customize
		InlineContentNonce string
		MaxSecretExpiry    int64
		Version            string
	}{
		Customize:          cust,
		InlineContentNonce: inlineContentNonceStr,
		MaxSecretExpiry:    cfg.SecretExpiry,
		Version:            version,
	}); err != nil {
		logrus.WithError(err).Error("executing index template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func handleRemoveAcceptEncoding(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Accept-Encoding")
		next.ServeHTTP(w, r)
	})
}
