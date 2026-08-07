# OTS (One-Time Secrets) Change Log

All notable changes to **OTS (One-Time Secrets)** will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.42.0] - 2026-08-07

### Security & Validation
- **API-Level Extension Filtering (`api.go`, `api_test.go`):**
  - Enforced server-side file extension validation directly in `POST /api/create` handler, eliminating security risks from unauthenticated API requests attempting to bypass Vue frontend validation.
  - Automatically decodes unencrypted `"OTS1"` metadata containers to inspect attached filenames, cross-referencing against configured `cust.AcceptedFileTypes` and group aliases (`@images`, `@office`, `@security`).
  - Immediately rejects invalid file uploads with HTTP `400 Bad Request` and `{"success": false, "error": "file_extension_not_allowed"}`.

### Added
- **Pluggable Storage Backends (`pkg/storage/factory`, `pkg/storage/memcached`, `pkg/storage/sqlite`, `pkg/storage/badger`):**
  - **Unified Storage Factory (`pkg/storage/factory`):** Introduced connection URI engine supporting `memory://`, `redis://`, `memcached://`, `sqlite://`, and `badger://`.
  - **Memcached Distributed Engine (`pkg/storage/memcached`):** Integrated 100% pure Go `github.com/bradfitz/gomemcache` driver with Compare-And-Set (CAS) atomic read-and-destroy semantics.
  - **SQLite Relational Engine (`pkg/storage/sqlite`):** Integrated 100% CGO-free `modernc.org/sqlite` pure Go driver (`sqlite:///path/to/ots.db` or `sqlite://:memory:`) with WAL journal mode, busy timeouts, and background expiration cleanup ticker.
  - **BadgerDB LSM Engine (`pkg/storage/badger`):** Integrated 100% CGO-free `github.com/dgraph-io/badger/v4` high-performance log-structured merge-tree database (`badger:///path/to/db`) utilizing native entry TTL expiration (`WithTTL`) and ACID value log garbage collection.

---

## [v1.41.0] - 2026-08-06

### Added
- **Sender Context Message Subsystem (`src/ots-meta.js`, `src/components/create.vue`, `src/components/secret-display.vue`):**
  - **Zero-Knowledge Encapsulation (`src/ots-meta.js`):** Extended `OTSMeta` serialization format to encapsulate optional context messages (up to 200 characters) and format specifiers (`text`, `md`, `html`, `json`), keeping all note content encrypted client-side via WebCrypto AES-256-GCM before server posting while preserving backward compatibility for legacy secret payloads.
  - **Sender Modal & Format Selector (`src/components/create.vue`):** Added `[ + Add a Message ]` / `[ ✓ Message Attached ]` header action button opening a format selector modal (`Plain Text`, `Markdown`, `HTML`, `JSON`), live character counter with visual warning thresholds (`0 / 200`), and interactive `[ 🪄 Sample Template ]` starter templates.
  - **Unmodified Template Auto-Discard (`src/components/create.vue`):** Added automatic detection in `saveMessageModal` to discard unedited default starter templates or empty note inputs upon saving, preventing accidental template attachment.
  - **In-Modal Clear Note (`src/components/create.vue`):** `[ Clear Note ]` resets textarea input content while remaining active inside the modal dialog.
  - **Recipient Display Container (`src/components/secret-display.vue`):** Added `[ ✉️ Message Received ]` header badge and `Sender Context Note` display card featuring format pill badge, monospaced text rendering, parsed Markdown/HTML container, and dedicated `[ Copy note to clipboard ]` button.
  - **ZIP Bundle Integration (`src/components/secret-display.vue`):** Updated `downloadBundle()` to automatically package context notes (`note.txt`, `note.md`, `note.html`, or `note.json`) inside generated `.zip` bundle archives alongside `secret.txt`, attachments, and `SHA256SUMS`.

### Security
- **Dual-Sided DOMPurify XSS Protection (`src/components/create.vue` & `src/components/secret-display.vue`):**
  - Integrated `dompurify` (v3.4.13) security library on both sender creation (`saveMessageModal`) and recipient rendering (`secret-display.vue`) for complete defense-in-depth against Stored XSS attacks.
  - Enforced strict zero-active-content policy: automatically purges `<script>`, `<iframe>`, `<object>`, `<embed>`, `<style>`, `<form>`, `<svg>`, inline event handlers (`onload=`, `onerror=`, `onclick=`), style expressions, and dangerous URI schemes (`javascript:`, `data:`, `vbscript:`).

---

## [v1.40.0] - 2026-08-01

### Added
- **Authentication & Authorization Subsystem (`pkg/auth`):**
  - **ForwardAuth Reverse Proxy Connector (`pkg/auth/forwardauth.go`):** Implemented session header trust for Authelia, Authentik, OAuth2-Proxy, Pomerium, and Okta with anti-spoofing CIDR/IP validation (`trustedProxies`).
  - **Local Argon2id Authenticator (`pkg/auth/local.go`):** Implemented OWASP-compliant Argon2id password hashing (`m=64MB, t=3, p=4`), base64 fallback decoding, atomic `.tmp` user file saving, `DeleteUser()` functionality, and automatic hot-reloading on disk changes (`users.yaml`).
  - **Group-Based RBAC Policy Evaluator (`pkg/auth/rbac.go`):** Implemented group membership authorization (`allowedGroups`) on protected endpoints like `POST /api/create` with `path.Clean()` trailing slash traversal normalization.
  - **HTTP Auth Middleware (`pkg/auth/middleware.go`):** Added authentication middleware returning standardized `401 Unauthorized` and `403 Forbidden` JSON error responses (`Content-Type: application/json; charset=utf-8`).
  - **Identity Configuration Unmarshaler (`pkg/auth/identity.go`):** Added `LoadIAMConfig` supporting both top-level `iam:` YAML wrapper blocks and root schema structures (`iam.yaml`).
- **Administrative CLI User Management Command Suite (`cmd/ots-cli/cmd_user.go`):**
  - Added `ots-cli user add`, `list`, `disable`, and `delete` commands with onboarding credential template generation.
- **Orchestration Suite Enhancements (`tools/ots_builder.sh`):**
  - Added `--with-redis [<path>]` supporting optional binary path parameters, automatic `$PATH` prepending, `$REDIS_HOME` environment inspection, and clean exit aborts on missing binaries.
  - Added background Redis server lifecycle orchestration (`start_redis_server`, `stop_redis_server`) for live API validation (`--validate`) on port `63799`.
  - Added `-h` / `--help` CLI flag displaying interactive usage menu (`show_usage`).

---

## [v1.31.0] - 2026-07-31

### Added
- **Enterprise Delivery Message Generator & Modal (`src/components/message-modal.vue` & `src/components/display-url.vue`):**
  - Added high-contrast `[Generate Message]` button (`btn btn-success btn-sm text-white shadow-sm fw-semibold`) on the right side of the *"Secret created"* card banner header.
  - Added 4 enterprise delivery template tabs (**Full Link**, **Dual Link Part 1**, **Dual Key Part 2**, **Combined Chat**).
  - Added an interactive **Format Switcher** supporting **Plain Text (ASCII Box)**, **HTML (Rich Email Box)**, **Markdown (Slack/Teams)**, and **JSON (API/Webhooks)** with 1-click copy buttons.
  - Unit test `TestEnterpriseMessageTemplatesRendering` added in `api_test.go` verifying zero emojis and ASCII box-drawing border integrity.
- **Structured File Logging & NDJSON Support (`main.go`):**
  - Added `--log-file-path` flag to redirect log output to a dedicated log file path.
  - Added `--log-format` flag supporting `text`, `json`, and `ndjson` (Newline Delimited JSON) structured logging formats.
  - Added unit tests `TestLogFormatNDJSON` and `TestLogFilePathWriting` in `main_test.go`.
- **Security Extension Groups & Merging (`pkg/customization/file_groups.go`):**
  - Added `@security-files` and `@security` extension aliases (`.pem, .crt, .key, .cer, .pfx, .asc, .jks, .p12, .der, .csr, .crl`).
  - Updated `ExpandAcceptedFileTypes` to append/merge custom JSON group extension definitions with built-in default groups.
  - Added unit test `TestExpandAcceptedFileTypes` in `file_groups_test.go`.

---

## [v1.30.0] - 2026-07-30

### Added
- **Group-Based File Extension Management Subsystem (`pkg/customization/file_groups.go`):**
  - Replaced browser/OS-dependent `file.type` MIME detection with deterministic extension group aliases (`@images`, `@office`, `@archives`, `@packages`, `@binaries`, `@code`, `@video`, `@audio`).
  - Added support for loading custom group mappings from external JSON files (`fileGroupsPath`) and compound extension matching (e.g. `.tar.gz`).
- **Dual-Channel Secret Delivery & Decryption (Issue #208):**
  - Implemented `client.SplitSecretURL(unifiedURL)` and `client.FetchWithKey(baseURL, key)` in `pkg/client` allowing separate channel transmission of Base Secret URLs (`#secret_id`) and Decryption Keys.
- **Search Engine Privacy & Indexing Protection (Issue #221):**
  - Added configurable `disableSearchIndex` setting (defaults to `true`), delivering `Disallow: /` on `/robots.txt` and `X-Robots-Tag: noindex` HTTP response headers.
- **Cumulative Storage Cap Enforcement (Issue #234):**
  - Enforced `maxAttachmentSizeTotal` cumulative instance attachment storage limit checks on `/api/create`, returning HTTP 507 Insufficient Storage when total active secret storage is exhausted.
- **Live Server E2E & CLI Integration Test Suite (`cmd/ots-cli/cli_e2e_test.go` & `api_test.go`):**
  - Added live local HTTP server E2E test cases validating 10MB binary attachment transfer, group alias extension filtering, sliding-window IP rate limiting, and dual-channel URL splitting against an active Gorilla Mux router instance.
- **System Documentation (`PRODUCT.md`, `ARCHITECTURE.md`, `TESTING.md`, `IMPROVEMENTS.md`):**
  - Published operational guides, security assessment reports, Mermaid architecture diagrams, test specification tables, and CLI flags inventory using relative Markdown links.

### Changed
- **Pointer Receivers in `api.go` (`govet` `copylocks` Fix):**
  - Converted `apiServer` method receivers from value pass `(a apiServer)` to pointer receivers `(a *apiServer)` to prevent copying atomic/mutex primitives by value.
- **Context Propagation in Tests:**
  - Converted 20+ `httptest.NewRequest(...)` calls to `httptest.NewRequestWithContext(context.Background(), ...)` to satisfy `noctx` static analysis rules.

### Security
- **Static Analysis & Vulnerability Remediation:**
  - Fixed 27 GolangCI Meta-Linter warnings across `errcheck`, `noctx`, `testifylint`, `gofumpt`, and `strings` imports.
  - Added explicit justification comments and `<!-- nosemgrep -->` directives for trusted `i18n.yaml` and `customize.yaml` HTML rendering in Vue components.
  - Scoped `IFS=,` array joins to local subshells in shell automation scripts (`ci/docker-gen-tagnames.sh`).
  - Audited dependencies: 0 secrets (TruffleHog), 0 security issues (Gosec), 0 NPM vulnerabilities (`package-lock.json` synchronized), 0 module vulnerabilities (GoVulnCheck).

---

For historical release records prior to v1.30.0, see [History.md](History.md).
