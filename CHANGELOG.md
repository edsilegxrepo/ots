# OTS (One-Time Secrets) Change Log

All notable changes to **OTS (One-Time Secrets)** will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

## [v1.21.9] - 2026-07-25
### Fixed
- Updated `github.com/prometheus/client_golang` module to v1.24.1.
- Updated `vue-i18n` dependency to v11.4.8.
- Updated `eslint` toolchain to v10.8.0.

---

## [v1.21.8] - 2026-07-04
### Maintenance
- Updated GitHub Actions dependencies and base Docker images.
- Upgraded Valkey/Redis container image tag to v9.1.1.

---

## [v1.21.7] - 2026-06-27
### Fixed
- Updated `github.com/prometheus/client_golang` module to v1.24.0.
- Updated `vue-i18n` to v11.4.7.

---

## [v1.21.6] - 2026-06-14
### Maintenance
- Minor dependency updates and CI build pipeline maintenance.

---

## [v1.21.5] - 2026-04-24
### Maintenance
- Routine dependency updates and security vulnerability patches.

---

## [v1.21.4] - 2026-04-07
### Maintenance
- Updated Go toolchain and frontend asset build scripts.

---

## [v1.21.3] - 2026-03-15
### Maintenance
- Dependency vulnerability scans and linting rule updates.

---

## [v1.21.2] - 2026-02-20
### Maintenance
- Dependency updates for Go modules and npm package manifests.

---

## [v1.21.1] - 2026-02-16
### Maintenance
- Maintenance patch release for build configuration and dependencies.

---

## [v1.21.0] - 2026-01-20
### Added
- OpenSSL PBKDF2 iteration parameter alignments for high-security key derivation.

---

## [v1.20.1] - 2025-12-19
### Fixed
- Bug fixes for edge-case URL fragment unescaping in client SDK.

---

## [v1.20.0] - 2025-11-14
### Added
- Expanded file attachment metadata support and client pre-flight checks.

---

## [v1.19.1] - 2025-10-18
### Maintenance
- Bug fixes and translation updates.

---

## [v1.19.0] - 2025-10-15
### Added
- Custom operator banner HTML rendering and formal language toggles.

---

## [v1.18.0] - 2025-08-13
### Added
- Prometheus metrics collector integration and `/metrics` HTTP endpoint.

---

## [v1.17.3] - 2025-08-08
### Maintenance
- Patch release updating frontend UI styling and dependencies.

---

## [v1.17.2] - 2025-06-15
### Maintenance
- Maintenance updates for Redis storage backend connection handling.

---

## [v1.17.1] - 2025-05-29
### Maintenance
- Fixed minor translation string key mappings.

---

## [v1.17.0] - 2025-05-12
### Added
- Initial support for customizable secret expiry choices in `customize.yaml`.

---

## [v1.16.0] - 2025-05-01
### Added
- Standalone OTS CLI utility (`cmd/ots-cli`) for terminal secret creation and fetching.

---

## [v1.15.1] - 2024-12-12
### Maintenance
- Fixed static asset asset delivery header caching.

---

## [v1.15.0] - 2024-12-05
### Added
- WebCrypto API adoption for fast client-side browser secret encryption.

---

## [v1.14.0] - 2024-11-21
### Added
- Attachment support in `pkg/client` OTSMeta payload format.

---

## [v1.13.0] - 2024-08-27
### Added
- Sliding window IP rate limiter implementation for creation endpoints.

---

## [v1.12.0] - 2024-01-24
### Added
- Customization configuration support (`customize.yaml`).

---

## [v1.11.1] - 2023-12-12
### Fixed
- Patch release resolving minor UI asset embedding issues.

---

## [v1.11.0] - 2023-12-10
### Added
- Redis storage engine provider (`pkg/storage/redis`).

---

## [v1.10.0] - 2023-11-11
### Added
- Initial release of OTS with zero-knowledge in-memory secret storage.
