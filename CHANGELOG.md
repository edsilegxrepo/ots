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

For historical release records prior to v1.30.0, see [History.md](History.md).
