# OTS (One-Time Secrets) Enhancement & Integration Plan

This document outlines the selected GitHub issues and repository improvements planned for implementation in **OTS**.

---

## Roadmap Overview

| Phase | Category | Issue / Feature | Key Focus |
|---|---|---|---|
| **Phase 1** | **File Handling** | **Group-Based Extension Management** *(Top Priority)* | Extension groups (`@images`, `@archives`, `@office`, `@packages`, `@binaries`, etc.), OS/browser-independent validation via JSON config |
| **Phase 1** | **Privacy** | Issue #221 (`robots.txt`) | Disable search engine indexing by default with optional configuration |
| **Phase 1** | **UX Polish** | Issue #196 (Misc UI Improvements) | QR code tooltips, modal popup for "How it works", custom UI text |
| **Phase 1** | **UX / Config** | Issues #153 & #183 (Expiry UI) | Display explicit default expiry duration and support `disableDefaultExpiry` |
| **Phase 2** | **Security / Ops**| **Server-Side API Protection** *(Single Binary)* | Rate limiting per IP, server-side payload & extension validation in Go |
| **Phase 2** | **Security / Ops**| Issue #234 (Storage Cap) | Configurable total attachment disk usage limit for DoS prevention |
| **Phase 2** | **Security** | Issue #208 (Dual-Channel Delivery)| Split secret URL and decryption key for separate channel transmission |

---

## Detailed Phase Specifications

### Phase 1: Quick Wins, UX Polish & Extension Improvements

#### 1. Group-Based File Extension & Management System *(TOP PRIORITY)*
- **Problem with Current System:**
  - **Browser/OS Dependency Flaw:** Relying on `file.type` (MIME types returned by `File.type` in JavaScript) is flaky because MIME types depend on local OS registry entries across Windows, Linux, macOS, iOS, and Android.
  - **Unfriendly Config:** Administrators must manually type out dozens of individual file extensions or unstandardized MIME strings.
  - **Case/Format Sensitivity:** Extension checks fail for upper case (`.PDF`), numeric extensions (`.7z`, `.mp4`), or formats missing leading dots (`png`).

- **New Architectural Solution:**
  - **JSON Config File for File Groups (`file_groups.json`):**
    Pre-define standard extension groups that can be enabled by group aliases in `customize.yaml` or extended via external JSON:
    - `@archives`: `.zip`, `.7z`, `.rar`, `.tar`, `.gz`, `.tgz`, `.bz2`, `.xz`, `.iso`, `.cab`, `.zst`
    - `@images`: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.svg`, `.bmp`, `.ico`, `.tiff`, `.heic`
    - `@video`: `.mp4`, `.mkv`, `.avi`, `.mov`, `.wmv`, `.webm`, `.flv`, `.m4v`, `.3gp`
    - `@audio`: `.mp3`, `.wav`, `.ogg`, `.flac`, `.aac`, `.m4a`, `.wma`, `.opus`
    - `@office` / `@documents`: `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`, `.odt`, `.ods`, `.odp`, `.txt`, `.rtf`, `.csv`
    - `@packages`: `.deb`, `.rpm`, `.apk`, `.msi`, `.pkg`, `.appimage`, `.dmg`, `.flatpakref`, `.snap`, `.ipa`
    - `@binaries`: `.exe`, `.bin`, `.dll`, `.so`, `.dylib`, `.elf`, `.dat`
    - `@code`: `.json`, `.xml`, `.yaml`, `.yml`, `.py`, `.js`, `.ts`, `.go`, `.sh`, `.ps1`, `.html`, `.css`, `.sql`
  - **OS-Independent Filename Extension Validation:**
    - Parse extensions directly from the filename (e.g. `my_file.TAR.GZ` -> `.tar.gz`, `package.rpm` -> `.rpm`, `document.PDF` -> `.pdf`).
    - Eliminate reliance on browser/OS MIME detection (`file.type`).
  - **HTML `<input accept="...">` Integration:**
    - Expand group aliases (`@images, @office, @packages, .zip`) on the server into a clean HTML `accept` attribute string containing normalized file extensions (e.g. `.png,.jpg,.jpeg,.pdf,.docx,.deb,.rpm,.apk,.zip`).
  - **Unified Go & Vue Validator:**
    - Shared group resolution logic between Go backend (`pkg/client/sanity.go`, `pkg/customization`) and Vue frontend (`src/components/create.vue`).

---

#### 2. Issue #221: Search Engine Indexing Control (`robots.txt`)
- **Problem:** Search engines index public OTS instances, potentially exposing instance URLs and metadata.
- **Solution:**
  - Serve a default `robots.txt` disallowing all indexing (`User-agent: * \n Disallow: /`).
  - Add configuration setting `disableSearchIndex` in `customize.yaml` (defaulting to `true`).

---

#### 3. Issue #196: Miscellaneous UI Improvements
- **Problem:**
  - QR code mouseover tooltip is blank.
  - Clicking "How does this work" disrupts current user input.
- **Solution:**
  - Set QR code tooltip to `"QR Code for secret URL"`.
  - Convert "How does this work" into a modal popup overlay.
  - Add optional custom message field near secret creation dialog.

---

#### 4. Issues #153 & #183: Expiry Choices UI & Customization
- **Problem:**
  - "Default" expiry option doesn't show the actual time duration.
  - Instances with custom `expiryChoices` cannot hide the default dropdown entry.
- **Solution:**
  - Display duration text next to "Default" option (e.g., `Default (7 days)`).
  - Add `disableDefaultExpiry` config option to suppress default dropdown entry when custom choices exist.

---

### Phase 2: Security, Single-Binary API Protection & Abuse Prevention

#### 5. Native Single-Binary API Protection
- **Context:** OTS runs as a single Go binary serving both the frontend SPA and `/api/*` endpoints from the same origin.
- **Key Enhancements:**
  - **Server-Side Extension Enforcement in `api.go`:** Inspect secret payloads in `/api/create` on the server to enforce allowed extension groups even if requests bypass the Vue UI.
  - **In-Memory Rate Limiting:** Add an IP-based token bucket middleware in Go for `/api/create` (e.g. max 10 creations/min per IP) to block automated brute-force / spam creation.

---

#### 6. Issue #234: Total Attachment Disk Usage Cap
- **Problem:** Spammers/malicious actors can fill server disk storage with large attachments.
- **Solution:**
  - Introduce `max-total-attachment-size` / `maxAttachmentSizeTotal` setting.
  - Reject new file attachment uploads once cumulative storage reaches the defined limit.

---

#### 7. Issue #208: Separated Decryption Key / Dual-Channel Delivery
- **Problem:** Sharing full secret URLs in a single communication channel poses security risks.
- **Solution:**
  - Allow secret links to omit the decryption key fragment.
  - Prompt the recipient for the decryption passphrase/key when opened, allowing separate delivery via secondary channels (SMS, Signal, Slack).

---

#### 8. Major Enhancement: High-Capacity Attachment Support (>64MB Memory Optimization)
- **Problem:** Historical 64MB frontend attachment limits were caused by V8 call-stack overflows when unpacking large binary arrays (`[...uint8Array]`) and monolithic string memory allocations in `btoa()`.
- **Solution:**
  - Replaced array-spreading call-stack operations in `src/crypto.ts` with TypedArray direct zero-copy `set()` buffer operations.
  - Enabled memory-efficient chunked base64 processing via `base64-js`.
  - Configurable server-side storage and secret size limits (`maxSecretSize` and `maxAttachmentSizeTotal`) allowing administrators to safely host larger attachments (256MB, 512MB, 1GB+).

---

## 📌 Implementation Checklist

- [x] **1. Group-Based File Extension Management (`file_groups.json` & OS-independent validation)**
- [x] **2. Search Engine Indexing (#221)**
- [x] **3. UI Improvements (#196)**
- [x] **4. Expiry UI Polish (#153, #183)**
- [x] **5. Single-Binary Server-Side API Protection & Rate Limiting**
- [x] **6. Total Attachment Storage Cap (#234)**
- [x] **7. Dual-Channel Key Separation (#208)**
- [x] **8. High-Capacity Attachment Support (>64MB Memory Optimization)**
