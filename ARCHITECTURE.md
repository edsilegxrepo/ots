# OTS (One-Time Secrets) Architecture & System Design

This document details the software architecture, design choices, operational data flows, concurrency models, dependency graphs, and security controls of **OTS (One-Time Secrets)**.

---

## 1. Architecture and Design Choices, Assumptions, Edge Cases, Performance & Efficiency

### System Architecture Diagram
```mermaid
graph TD
    subgraph "Client Layer (Zero-Knowledge)"
        WebUI["Vue SPA Web Client<br/>(WebCrypto AES-256-CBC)"]
        CLIClient["Go CLI Client<br/>(cmd/ots-cli)"]
        SDKClient["Go SDK Engine<br/>(pkg/client)"]
    end

    subgraph "Network & Ingestion Layer"
        TLS["HTTPS / TLS 1.3 Termination"]
        Mux["Gorilla Mux HTTP Router<br/>(api.go / main.go)"]
        RateLimiter["Sliding Window IP Rate Limiter<br/>(ratelimit.go)"]
        AuthMiddleware["Identity & RBAC Middleware<br/>(pkg/auth)"]
    end

    subgraph "Identity & Auth Subsystem (pkg/auth)"
        ForwardAuth["ForwardAuth Proxy Connector<br/>(pkg/auth/forwardauth.go)"]
        LocalArgon2["Local Argon2id Authenticator<br/>(pkg/auth/local.go)"]
        RBACEvaluator["Group RBAC Policy Engine<br/>(pkg/auth/rbac.go)"]
    end

    subgraph "Core Engine Layer"
        APIServer["APIServer REST Controller<br/>(api.go)"]
        CapEngine["Cumulative Storage Cap Evaluator<br/>(maxAttachmentSizeTotal)"]
        CustEngine["Customization & Extension Manager<br/>(pkg/customization)"]
        MetricsVec["Prometheus Collector Vectors<br/>(pkg/metrics)"]
    end

    subgraph "Storage Abstraction Layer"
        StoreIntf["Storage Interface<br/>(pkg/storage/storage.go)"]
        StoreFactory["Unified Storage Engine Factory<br/>(pkg/storage/factory)"]
        MemEngine["In-Memory Store<br/>(pkg/storage/memory)"]
        RedisEngine["Redis KV Store<br/>(pkg/storage/redis)"]
        MemcachedEngine["Memcached CAS Store<br/>(pkg/storage/memcached)"]
        SQLiteEngine["Pure Go SQLite Engine<br/>(pkg/storage/sqlite)"]
        BadgerEngine["BadgerDB LSM Engine<br/>(pkg/storage/badger)"]
    end

    WebUI -->|"POST /api/create (Encrypted)"| TLS
    CLIClient -->|"POST /api/create (Encrypted)"| TLS
    SDKClient -->|"POST /api/create (Encrypted)"| TLS

    TLS --> Mux
    Mux --> RateLimiter
    RateLimiter --> AuthMiddleware
    AuthMiddleware --> ForwardAuth
    AuthMiddleware --> LocalArgon2
    ForwardAuth --> RBACEvaluator
    LocalArgon2 --> RBACEvaluator
    RBACEvaluator -->|"Authorized"| APIServer

    APIServer --> CapEngine
    APIServer --> CustEngine
    APIServer --> MetricsVec
    APIServer --> StoreIntf

    StoreIntf --> MemEngine
    StoreIntf --> RedisEngine
```

### Architectural Design Choices & Assumptions:
- **Zero-Knowledge Encryption:** The server acts purely as an untrusted key-value broker. Plaintext secrets and decryption keys exist exclusively within client RAM and WebCrypto memory spaces.
- **URL Fragment Passphrase Storage:** The encryption passphrase is stored in the URL fragment (`http://...#secret_id|key`). RFC 3986 guarantees that fragments are stripped by HTTP user-agents and never transmitted to the server or reverse proxies.
- **Atomic Read & Destroy:** Storage backends implement non-recoverable delete-on-read semantics (`ReadAndDestroy`). Once fetched, secret blobs are instantly deleted from RAM or Redis key spaces.

### Edge Cases & Defensive Safeguards:
1. **Misconfigured Proxy / Enormous Payload Safeguard:** `http.MaxBytesReader` cuts client connections if payloads exceed `2 * MaxSecretSize` to protect server memory.
2. **Path Traversal & Filename Disambiguation:** Attachment filenames with relative path elements (`../`, `..\`) or dangerous characters are stripped to clean basenames (`filepath.Base`), and filename collisions automatically receive numeric index suffixes.
3. **Storage Exhaustion DoS Mitigation:** In addition to individual payload caps, `maxAttachmentSizeTotal` tracks cumulative active memory bytes atomically to return HTTP 507 Insufficient Storage when total disk/RAM quotas are reached.

---

## 2. Data Flow and Control Logic

### Operational Data Flow Sequence
```mermaid
sequenceDiagram
    autonumber
    actor Sender as Sender Client
    actor Receiver as Receiver Client
    participant SPA as Vue SPA / CLI / SDK
    participant API as OTS APIServer Controller
    participant RL as Sliding Window Rate Limiter
    participant Store as Storage Provider (Memory/Redis)

    rect rgb(230, 245, 230)
        note over Sender, Store: Secret Creation & Encryption Flow
        Sender->>SPA: Input secret text & file attachments
        SPA->>SPA: Generate CSPRNG 20-char random key & PBKDF2 salt
        SPA->>SPA: Encrypt payload locally via AES-256-CBC
        SPA->>RL: POST /api/create (Encrypted Base64 Payload)
        RL-->>API: Rate limit check OK (IP under limit)
        API->>Store: Create(encryptedBlob, duration)
        Store-->>API: Persist payload & return secret_id
        API-->>SPA: 201 Created (secret_id, expires_at)
        SPA-->>Sender: Return Secret Link: http://ots.local/#secret_id|key
    end

    rect rgb(235, 235, 255)
        note over Receiver, Store: Dual-Channel & Retrieval Flow
        Sender->>Receiver: Channel A: Send URL http://ots.local/#secret_id
        Sender->>Receiver: Channel B: Send Key (decryption_key)
        Receiver->>SPA: Open link & input key
        SPA->>API: GET /api/get/secret_id (Omit Key from HTTP request)
        API->>Store: ReadAndDestroy(secret_id)
        Store-->>API: Return encrypted blob & atomically delete entry
        API-->>SPA: 200 OK (Encrypted Base64 Payload)
        SPA->>SPA: Decrypt blob locally using Key
        SPA-->>Receiver: Display plaintext secret & attachments
    end
```

### Code Relationships & Components:
- **`main.go` & `helpers.go`**: Parses CLI flags via standard `flag` with `os.Getenv` fallbacks, configures loggers, sets up CORS and security headers via custom `CSP`, provides layered filesystem asset resolution (`fsStack`), serves gzip compression (`gzipMiddleware`) and request logging (`httpLoggerMiddleware`), embeds static web assets, and launches HTTP server listeners.
- **`api.go` (`APIServer`)**: Orchestrates `/api/create`, `/api/get/{id}`, `/api/settings`, `/healthz`, and `/isWritable`.
- **`ratelimit.go` (`ipRateLimiter`)**: Thread-safe sliding window rate limiter tracking request timestamps per client IP.
- **`pkg/auth`**: Decoupled Identity & Access Management subsystem providing `ForwardAuth` proxy header trust, `Local` Argon2id password verification (`users.yaml`), and `RBAC` policy evaluation (`allowedGroups`).
- **`pkg/customization` (`Customize`)**: Resolves operator settings, default expiry choices, and expands group extension aliases (`@images`, `@office`, `@archives`, `@packages`, `@binaries`).
- **`pkg/client` (`crypto.go`, `client.go`)**: Pure Go 1.26+ Cryptographic Client SDK providing OpenSSL-compatible AES-256-CBC + PBKDF2 encryption, constant-time PKCS7 unpadding, legacy MD5 KDF fallback, and programmatic `Create`, `Fetch`, `FetchWithKey`, and `SplitSecretURL` methods.
- **`cmd/ots-cli`**: Standalone CLI client providing secret note creation (`create`), retrieval (`fetch`), immediate destruction (`burn`), server settings & allowed extension queries (`info`), CSPRNG password generation (`genpass`), and `ots-cli user` user directory management (`add`, `list`, `disable`, `delete`).

---

## 3. Performance and Scalability

### Concurrency Model & Lock Granularity
- **Lock-Free Atomic Counters:** Active storage byte tracking (`storageBytes`) and Prometheus metrics vectors use Go lock-free `sync/atomic` primitives (`atomic.Int64`), eliminating mutex contention during high-throughput secret uploads and downloads.
- **32-Bucket Sharded Rate Limiting:** IP rate limiting (`ipRateLimiter`) shards request tracking across 32 independent `sync.Mutex` buckets using a zero-allocation inline FNV-1a integer hash (`uint32` shift-multiplier loop), completely eliminating global mutex lock contention under parallel request spikes.
- **Pre-Parsed Proxy CIDR Evaluation:** Trusted proxy IP validation (`cust.TrustedProxies`) pre-compiles CIDR blocks (`ResolvedTrustedCIDRs`) and IP ranges (`ResolvedTrustedIPs`) during startup, ensuring zero runtime string parsing during HTTP IP extraction.
- **Throttled Telemetry Updates:** Secret metrics calculation operates strictly on background 1-minute ticker routines, protecting backend storage engines (such as Redis) from unthrottled key scanning overhead under heavy API traffic.
- **Horizontal Scaling via Redis:** When configured with `--storage-type=redis`, OTS operates completely stateless, allowing horizontal scaling behind load balancers with native Redis key TTL eviction.

---

## 4. Dependencies & System Modules

```mermaid
graph TD
    subgraph "Core Binaries & Submodules"
        MainModule["github.com/edsilegxrepo/ots<br/>(Main Server Binary)"]
        CLIModule["cmd/ots-cli<br/>(Standalone CLI Utility)"]
        ClientModule["pkg/client<br/>(Cryptographic Client SDK)"]
        CustModule["pkg/customization<br/>(Customization & Extensions)"]
        MetricsModule["pkg/metrics<br/>(Prometheus Telemetry)"]
        StorageMem["pkg/storage/memory<br/>(In-Memory Store)"]
        StorageRedis["pkg/storage/redis<br/>(Redis Distributed Store)"]
        StorageMemcached["pkg/storage/memcached<br/>(Memcached Distributed Store)"]
        StorageSQLite["pkg/storage/sqlite<br/>(Pure Go SQLite Engine)"]
        StorageBadger["pkg/storage/badger<br/>(BadgerDB LSM Engine)"]
        StorageFactory["pkg/storage/factory<br/>(Unified Engine Factory)"]
    end

    subgraph "Standard Library & Third-Party Dependencies"
        Mux["github.com/gorilla/mux<br/>(HTTP Router v1.8.1)"]
        StdCrypto["golang.org/x/crypto/pbkdf2<br/>(Native OpenSSL PBKDF2 / AES-256-CBC)"]
        Prometheus["github.com/prometheus/client_golang<br/>(Metrics v1.24.1)"]
        GoRedis["github.com/redis/go-redis/v9<br/>(Redis Client v9.22.0)"]
        SQLiteDriver["modernc.org/sqlite<br/>(Pure Go CGO-Free SQLite v1.56.0)"]
        BadgerDriver["github.com/dgraph-io/badger/v4<br/>(BadgerDB LSM v4.9.6)"]
        MemcachedDriver["github.com/bradfitz/gomemcache<br/>(Memcached Client)"]
        Logrus["github.com/sirupsen/logrus<br/>(Structured Logging v1.9.4)"]
        UUID["github.com/gofrs/uuid<br/>(UUID Generator v4.4.0)"]
        Testify["github.com/stretchr/testify<br/>(Testing Toolkit v1.11.1)"]
    end

    MainModule --> Mux
    MainModule --> Logrus
    MainModule --> UUID
    MainModule --> CustModule
    MainModule --> MetricsModule
    MainModule --> StorageFactory
    StorageFactory --> StorageMem
    StorageFactory --> StorageRedis
    StorageFactory --> StorageMemcached
    StorageFactory --> StorageSQLite
    StorageFactory --> StorageBadger
    StorageSQLite --> SQLiteDriver
    StorageBadger --> BadgerDriver
    StorageMemcached --> MemcachedDriver

    ClientModule --> OpenSSL
    ClientModule --> Logrus

    CLIModule --> ClientModule

    MetricsModule --> Prometheus
    StorageRedis --> GoRedis
    StorageMem --> Testify
```

---

## 5. Security Architecture & RBAC

```mermaid
graph TD
    subgraph "Edge & Transport Security"
        HTTPS["HTTPS / TLS 1.2+ Termination"]
        HeaderSec["Security Headers<br/>(Cache-Control: no-store, X-Robots-Tag: noindex)"]
    end

    subgraph "Ingestion & Rate Limiting Gate"
        RateLimitGate["IP Rate Limiter Gate<br/>(30 requests / minute per IP)"]
        PayloadGate["Payload Boundary Gate<br/>(MaxBytesReader & maxAttachmentSizeTotal)"]
        ExtFilterGate["Extension Policy Gate<br/>(Group Aliases: @images, @office)"]
    end

    subgraph "Access Control & Endpoint RBAC"
        PublicEndpoints["Public Unauthenticated Endpoints<br/>- POST /api/create<br/>- GET /api/get/{id}<br/>- GET /api/settings<br/>- GET /healthz"]
        RestrictedMetrics["Restricted Telemetry Endpoint<br/>- GET /metrics"]
        SubnetFilter["CIDR Subnet Whitelist Filter<br/>(metricsAllowedSubnets)"]
    end

    HTTPS --> HeaderSec
    HeaderSec --> RateLimitGate
    RateLimitGate --> PayloadGate
    PayloadGate --> ExtFilterGate

    ExtFilterGate --> PublicEndpoints
    ExtFilterGate --> SubnetFilter
    SubnetFilter -->|Authorized IP| RestrictedMetrics
    SubnetFilter -->|Unauthorized IP| Deny["403 Forbidden"]
```

### Access Control & Security Policies:
1. **Unauthenticated Public Interface:** Secret submission (`POST /api/create`) and secret retrieval (`GET /api/get/{id}`) operate unauthenticated to ensure zero-friction encrypted secret exchanges.
2. **Telemetry Whitelist RBAC (`metricsAllowedSubnets`):** Access to Prometheus metrics (`GET /metrics`) is enforced via CIDR subnet matching (e.g. `["127.0.0.1/32", "10.0.0.0/8"]`). Unauthorized IPs receive HTTP 403 Forbidden.
3. **Zero Traceability:** Error responses generate a random tracking UUID (`err_id`) logged internally while returning clean JSON error responses to callers, preventing internal stack trace disclosure.

---

## 6. System References
- **[PRODUCT.md](PRODUCT.md)** - Operational Guide, Security Assessment & Deployment Options
- **[TESTING.md](TESTING.md)** - Complete Test Suite Specifications, E2E Specifications, and Code Coverage Report
