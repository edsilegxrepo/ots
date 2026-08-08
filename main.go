// The OTS Main Server Entrypoint
//
// Objectives:
// - Initializes OTS HTTP server configuration, CLI flags, TLS hardening, and storage drivers.
// - Bootstraps embedded Vue 3 frontend single-page application assets and CSP security policy.
// - Provides graceful server shutdown signal listeners for clean database and connection flushes.
//
// Core Components:
// - main: Server entrypoint initializing flags, storage, HTTP router, listener, and signal traps.
// - parseFlags: Command line parameter loader for storage drivers, ports, TLS certs, and logs.
// - hardenListener: Network socket hardening forcing loopback binding when TLS is unconfigured.
// - defaultCSP: Content Security Policy header builder protecting frontend script execution.
//
// Data Flow:
// 1. main() -> parseFlags() -> Instantiates storage driver (sqlite/badger/memory/redis/memcached).
// 2. Registers REST API routes and static asset handlers -> Listens on TCP port.
// 3. SIGTERM/SIGINT -> Gracefully shuts down HTTP server and closes DB handles.
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/edsilegxrepo/ots/pkg/auth"
	"github.com/edsilegxrepo/ots/pkg/customization"
	"github.com/edsilegxrepo/ots/pkg/metrics"
)

const (
	scriptNonceSize    = 32
	defaultLogFileMode = 0o644
	ExitSuccess        = 0
	ExitConfigError    = 2
	ExitStorageError   = 3
	ExitNetworkError   = 4
)

var (
	cfg struct {
		Customize      string `flag:"customize" default:"" description:"Customize-File to load"`
		Listen         string `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		LogRequests    bool   `flag:"log-requests" default:"true" description:"Enable request logging"`
		LogLevel       string `flag:"log-level" default:"info" description:"Set log level (debug, info, warning, error)"`
		LogFilePath    string `flag:"log-file-path" default:"" description:"Path to file for appending log output (e.g. /var/log/ots.log)"`
		LogFormat      string `flag:"log-format" default:"text" description:"Set log output format (text, json, ndjson)"`
		SecretExpiry   int64  `flag:"secret-expiry" default:"86400" description:"Maximum expiry of the stored secrets in seconds"`
		StorageType    string `flag:"storage-type" default:"mem" description:"Storage to use for putting secrets to"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Print version information and exit"`
		EnableTLS      bool   `flag:"enable-tls" default:"false" description:"Enable HTTPS/TLS"`
		CertFile       string `flag:"cert-file" default:"" description:"Path to the TLS certificate file"`
		KeyFile        string `flag:"key-file" default:"" description:"Path to the TLS private key file"`
		IAMConfigFile  string `flag:"iam-config" default:"" description:"Path to iam.yaml configuration file"`
	}

	assets   fsStack
	cust     customization.Customize
	indexTpl *template.Template

	version = "dev"
)

//go:embed frontend/*
var embeddedAssets embed.FS

func defaultCSP() CSP {
	c := CSP{}

	c.Add("base-uri", CSPSrcSelf)
	c.Add("default-src", CSPSrcNone)
	c.Add("connect-src", CSPSrcSelf)
	c.Add("font-src", CSPSrcSelf)
	c.Add("img-src", CSPSrcSelf)
	c.Add("img-src", CSPSrcSchemeData)
	c.Add("script-src", CSPSrcSelf)
	c.Add("style-src", CSPSrcSelf)

	return c
}

//nolint:revive // enableTLS parameter is a deliberate configuration control flag for socket hardening
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

func parseFlags() {
	flag.StringVar(&cfg.Customize, "customize", getEnvOrDefault("CUSTOMIZE", ""), "Customize-File to load")
	flag.StringVar(&cfg.Listen, "listen", getEnvOrDefault("LISTEN", ":3000"), "IP/Port to listen on")
	flag.BoolVar(&cfg.LogRequests, "log-requests", getEnvOrDefaultBool("LOG_REQUESTS", true), "Enable request logging")
	flag.StringVar(&cfg.LogLevel, "log-level", getEnvOrDefault("LOG_LEVEL", "info"), "Set log level")
	flag.StringVar(&cfg.LogFilePath, "log-file-path", getEnvOrDefault("LOG_FILE_PATH", ""), "Path to file for appending log output")
	flag.StringVar(&cfg.LogFormat, "log-format", getEnvOrDefault("LOG_FORMAT", "text"), "Set log output format")
	flag.Int64Var(&cfg.SecretExpiry, "secret-expiry", getEnvOrDefaultInt64("SECRET_EXPIRY", 86400), "Maximum expiry in seconds")
	flag.StringVar(&cfg.StorageType, "storage-type", getEnvOrDefault("STORAGE_TYPE", "mem"), "Storage engine type")
	flag.BoolVar(&cfg.VersionAndExit, "version", false, "Print version information and exit")
	flag.BoolVar(&cfg.EnableTLS, "enable-tls", getEnvOrDefaultBool("ENABLE_TLS", false), "Enable HTTPS/TLS")
	flag.StringVar(&cfg.CertFile, "cert-file", getEnvOrDefault("CERT_FILE", ""), "Path to TLS cert")
	flag.StringVar(&cfg.KeyFile, "key-file", getEnvOrDefault("KEY_FILE", ""), "Path to TLS key")
	flag.StringVar(&cfg.IAMConfigFile, "iam-config", getEnvOrDefault("IAM_CONFIG", ""), "Path to IAM config file")

	if !flag.Parsed() {
		flag.Parse()
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvOrDefaultBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvOrDefaultInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func initApp() error {
	parseFlags()

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
		// #nosec G302 -- Log file permissions 0644 are standard for server logging
		logFile, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultLogFileMode)
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
		return fmt.Errorf("creating sub-fs for assets: %w", err)
	}
	assets = append(assets, frontendFS)

	if cust.OverlayFSPath != "" {
		assets = append(fsStack{os.DirFS(cust.OverlayFSPath)}, assets...)
	}

	cfg.Listen = hardenListener(cfg.Listen, cfg.EnableTLS)

	return nil
}

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

	hdl := setupHTTPHandler(api)

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

	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		cancel()
		logrus.WithError(shutdownErr).Error("Server forced to shutdown")
		os.Exit(ExitNetworkError)
	}
	cancel()

	logrus.Info("Server stopped cleanly")
	os.Exit(ExitSuccess)
}

func setupHTTPHandler(api *APIServer) http.Handler {
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

	if cfg.IAMConfigFile != "" {
		iamData, err := os.ReadFile(cfg.IAMConfigFile)
		if err != nil {
			logrus.WithError(err).Fatalf("failed to read iam-config file '%s'", cfg.IAMConfigFile)
		}
		iamCfg, err := auth.LoadIAMConfig(iamData)
		if err != nil {
			logrus.WithError(err).Fatalf("failed to parse iam-config YAML '%s'", cfg.IAMConfigFile)
		}
		localAuth, err := auth.NewLocalAuthenticator(iamCfg.UsersFilePath)
		if err != nil {
			logrus.WithError(err).Warnf("failed to load local users file '%s'", iamCfg.UsersFilePath)
		}
		authMW, err := auth.NewAuthMiddleware(iamCfg, localAuth)
		if err != nil {
			logrus.WithError(err).Fatalf("failed to initialize IAM auth middleware")
		}
		hdl = authMW.Handler(hdl)
		logrus.WithFields(logrus.Fields{
			"iam_config": cfg.IAMConfigFile,
			"connector":  iamCfg.Connector,
		}).Info("IAM Authentication middleware enabled")
	}

	hdl = gzipMiddleware(hdl)
	if cfg.LogRequests {
		hdl = httpLoggerMiddleware(hdl)
	}

	return hdl
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
	policy.Add("script-src", CSPSrcNonce(inlineContentNonceStr))
	policy.Add("style-src", CSPSrcNonce(inlineContentNonceStr))

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
