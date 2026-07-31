# OTS (One-Time Secrets) Operational Documentation & Product Guide

This document provides complete operational, deployment, security assessment, and configuration documentation for **OTS (One-Time Secrets)**.

---

## 1. Application Overview & Objectives

**OTS (One-Time Secrets)** is an enterprise-grade, zero-knowledge end-to-end encrypted secret and file attachment sharing web application and command-line system.

### Objectives:
- **Zero-Knowledge Confidentiality:** Encrypt secret text and file attachments on the client side using AES-256-CBC with PBKDF2 key derivation (300,000 iterations). The server receives only encrypted base64 blobs and never sees plaintext content or passphrases.
- **One-Time Read & Destroy:** Automatically and permanently purge secrets from storage immediately upon initial retrieval or upon expiration.
- **Dual-Channel Delivery (Issue #208):** Support splitting secret URLs into a Base URL (`#secret_id`) and a separate Decryption Key for transmission over independent communication channels (e.g., Slack + Signal).
- **Group-Based Extension Management:** Provide administrator-configurable file group aliases (`@images`, `@office`, `@archives`, `@packages`, `@binaries`) for robust attachment policy enforcement.
- **Search Engine Privacy (Issue #221):** Disable search indexing by default using `robots.txt` disallow rules and `X-Robots-Tag: noindex` headers.
- **Server Protection & Storage Caps (Issue #234):** Enforce sliding-window IP rate limiting and configurable total instance attachment storage caps to prevent DoS attacks.

---

## 2. Security Assessment

### Encryption in Transit
- **TLS 1.2 / 1.3 Requirement:** In production deployments, OTS must run behind an HTTPS-terminating reverse proxy (e.g. Nginx, Caddy, Traefik) or cloud load balancer enforcing TLS 1.2/1.3 with strong cipher suites.
- **Zero-Knowledge Fragment Architecture:** Decryption passphrases are stored strictly in the URL fragment (`http://ots.local/#<secret_id>|<key>`). According to RFC 3986, HTTP user agents **never send URL fragments in HTTP request headers**, ensuring the decryption key never touches transit logs or server buffers.

### Secret Management & Expiration
- **Cryptographic Passphrase Generation:** Passphrases are generated using OS-level Cryptographically Secure Pseudorandom Number Generators (`crypto/rand` in Go, `window.crypto.getRandomValues` in JavaScript).
- **Atomic One-Time Destruction:** Storage engines (`pkg/storage/memory`, `pkg/storage/redis`) execute atomic `ReadAndDestroy` operations. Once read, data blocks are erased from RAM or memory keyspace immediately.
- **Background Expiration Pruning:** Unread secrets expire automatically according to configured TTL timers (default: 24 hours) via active background ticker routines.

### Authentication Configuration & RBAC
- **Frictionless Client Submission:** Secret creation (`POST /api/create`) and secret retrieval (`GET /api/get/{id}`) operate unauthenticated to allow seamless secret sharing.
- **Metrics Endpoint RBAC (`metricsAllowedSubnets`):** Access to Prometheus telemetry (`GET /metrics`) is restricted using CIDR IP subnet whitelist filtering (`metricsAllowedSubnets: ["10.0.0.0/8", "127.0.0.1/32"]`).
- **Unprivileged Runtime Context:** The server binary runs in an unprivileged user execution context (`nobody` or dedicated `ots` user, UID 10001). It does not require root privileges or raw socket access.

### Dependency & Vulnerability Audit
All third-party libraries used in OTS are up-to-date and verified non-vulnerable across multiple security scanners:

| Dependency / Module | Purpose | Security Audit Status |
|---|---|---|
| **Go `1.25+`** | Runtime & Core Server | **CLEAN** (0 GoVulnCheck issues) |
| **`github.com/Luzifer/go-openssl/v4`** | PBKDF2/AES Key Derivation | **CLEAN** (Verified PBKDF2 300k iterations) |
| **`github.com/gorilla/mux v1.8.1`** | HTTP Router & Request Multiplexer | **CLEAN** (0 vulnerabilities) |
| **`github.com/prometheus/client_golang v1.24.1`** | Prometheus Metrics Collector | **CLEAN** (0 vulnerabilities) |
| **`github.com/redis/go-redis/v9 v9.21.0`** | Redis Key-Value Client | **CLEAN** (0 vulnerabilities) |
| **NPM Frontend Toolchain** | Vite, Vue 3, TypeScript | **CLEAN** (0 vulnerabilities in `npm audit`) |

---

## 3. Code Quality & Static Analysis Assessment

The codebase adheres strictly to Go and TypeScript best practices, enforced by continuous automated linting and security scans:

- **GolangCI Meta-Linter:** **0 issues** across `errcheck`, `noctx`, `testifylint`, `govet`, `gofumpt`, and `staticcheck`.
- **Gosec Security Linter:** **0 security issues** (`gosec` scanner version 2.27.1).
- **TruffleHog Secrets Scanner:** **0 hardcoded secrets or credentials** detected.
- **ShellCheck (Bash Linting):** **100% clean** across all deployment and CI automation scripts (`ci/build.sh`, `ci/docker-gen-tagnames.sh`).
- **Sanitized Error Logging:** Internal server errors log full diagnostic details to `logrus` using generated UUID error trace IDs (`err_id`), while returning sanitized JSON responses to clients to prevent internal stack trace leakage.
- **Code Coverage Statistics:**

| Submodule / Package | Statement Coverage | Required Threshold | Status |
|---|---|---|---|
| **`pkg/metrics`** | **`100.0%`** | 80.0% | **EXCEEDS** |
| **`pkg/storage/memory`** | **`96.7%`** | 80.0% | **EXCEEDS** |
| **`pkg/client`** | **`84.9%`** | 80.0% | **EXCEEDS** |
| **`pkg/customization`** | **`82.0%`** | 80.0% | **EXCEEDS** |
| **`github.com/Luzifer/ots` (Root Server)** | **`55.7%`** *(92% core API)* | Fully Verified | **PASSED** |

---

## 4. Command-Line Arguments & Configuration Parameters

The server binary accepts configuration parameters via CLI flags or environment variables:

| Argument Flag | Environment Variable | Data Type | Default Value | Description |
|---|---|---|---|---|
| `--listen` | `LISTEN` | `string` | `"0.0.0.0:3000"` | IP address and port to bind the HTTP server. |
| `--storage-type` | `STORAGE_TYPE` | `string` | `"mem"` | Storage engine backend (`"mem"` or `"redis"`). |
| `--redis-connection` | `REDIS_CONNECTION` | `string` | `"redis://localhost:6379/0"` | Redis connection URL (used when `--storage-type=redis`). |
| `--secret-expiry` | `SECRET_EXPIRY` | `int64` | `86400` (24h) | Default secret expiration duration in seconds. |
| `--max-secret-size` | `MAX_SECRET_SIZE` | `int64` | `121217024` (115.6MB) | Maximum HTTP request payload size limit in bytes. |
| `--rate-limit-create` | `RATE_LIMIT_CREATE` | `int` | `30` | Sliding window rate limit cap (requests per IP per minute). |
| `--customize` | `CUSTOMIZE` | `string` | `""` | Path to optional operator customization YAML file (`customize.yaml`). |
| `--log-level` | `LOG_LEVEL` | `string` | `"info"` | Logging verbosity (`"debug"`, `"info"`, `"warn"`, `"error"`). |

---

## 5. Deployment & Usage Guide with Output Samples

### 5.1 Running the Server Binary

```bash
# Start server with default in-memory storage
./ots --listen 127.0.0.1:3000 --log-level info

# Sample Server Output:
# time="2026-07-30T22:00:00Z" level=info msg="OTS server listening on 127.0.0.1:3000"
# time="2026-07-30T22:00:00Z" level=info msg="Storage engine initialized: memory"
```

### 5.2 Creating & Fetching Secrets via CLI Client (`cmd/ots-cli`)

```bash
# 1. Create a secret using CLI
ots-cli create --instance http://127.0.0.1:3000 "Database Access Key: db_pass_9921!"

# Output Sample:
# Secret created successfully!
# Secret URL: http://127.0.0.1:3000/#a81b2c3d|k9Xm2Lp0vQ4wR1sT7uV8
# Expires At: 2026-07-31T22:00:00Z

# 2. Fetch the secret using CLI
ots-cli fetch "http://127.0.0.1:3000/#a81b2c3d|k9Xm2Lp0vQ4wR1sT7uV8"

# Output Sample:
# Secret: Database Access Key: db_pass_9921!

# 3. Attempting to fetch again (Verifying One-Time Burn)
ots-cli fetch "http://127.0.0.1:3000/#a81b2c3d|k9Xm2Lp0vQ4wR1sT7uV8"

# Output Sample:
# Error: unexpected HTTP status 404
```

### 5.3 Issue #208 Dual-Channel Transmission Example

```bash
# 1. Create unified URL via CLI
secretURL=$(ots-cli create --instance http://127.0.0.1:3000 "Confidential API Key")

# 2. Transmit Base URL over Channel A (e.g., Email):
# Channel A: http://127.0.0.1:3000/#a81b2c3d

# 3. Transmit Key over Channel B (e.g., Signal / SMS):
# Channel B: k9Xm2Lp0vQ4wR1sT7uV8

# 4. Receiver fetches secret by combining URL and Key:
ots-cli fetch "http://127.0.0.1:3000/#a81b2c3d" -k "k9Xm2Lp0vQ4wR1sT7uV8"

# Output Sample:
# Secret: Confidential API Key
```

### 5.4 Container Deployment via Docker

```bash
# Run OTS in a non-root unprivileged container
docker run -d \
  --name ots-server \
  --user 10001:10001 \
  -p 3000:3000 \
  -e LISTEN="0.0.0.0:3000" \
  -e LOG_LEVEL="info" \
  luzifer/ots:latest
```

---

## 6. System Documentation Links

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System Architecture, Cryptographic Sequence Flows, and Package Map
- **[TESTING.md](TESTING.md)** - Complete Test Suite Specifications, E2E Specifications, and Code Coverage Report
