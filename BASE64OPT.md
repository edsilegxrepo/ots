# Base64 Optimization Architecture & Concrete Implementation Blueprint (BASE64OPT.md)

Target Release: v1.50.0

This document provides the exhaustive, production-grade technical specification for Base64 handling, network payload efficiency, zero-allocation memory pooling, database storage normalization, and high-capacity streaming across OTS (One-Time Secrets).

---

## Executive Summary

While Base64 remains the standard encoding for web API compatibility and cross-platform interoperability, unoptimized implementations introduce payload inflation, memory allocation churn, and database bloat.

By implementing the 5 optimization pillars detailed below, OTS achieves:
1. 33.3% Storage Footprint Reduction across all 5 storage backends (Redis, Memcached, SQLite BLOB, BadgerDB, In-Memory RAM).
2. ~25% Network Payload Reduction on file attachments by eliminating double-Base64 encoding.
3. Zero URL Escaping Failures by standardizing on Unpadded URL-Safe Base64 (base64.RawURLEncoding / RFC 4648 Section 5).
4. Zero-Allocation Buffer Pooling in Go to eliminate garbage collection (GC) pause overhead under load.
5. 0% Encoding Overhead for large file attachments (>10 MB) via direct application/octet-stream streaming.

---

## 1. Pillar 1: Database Storage Layer Normalization ([]byte / BLOB)

### Technical Overview
Currently, client payloads are sent as Base64 ASCII strings and persisted into database engines directly as ASCII text strings. This wastes 33.3% of database memory, disk space, and Write-Ahead Log (WAL) bandwidth.

Resolution: Decode Base64 string to raw binary []byte at the API handler boundary in api.go before invoking the storage layer.

### Go Storage Interface Method Signatures (pkg/storage/storage.go)
```go
type Storage interface {
    // Create persists a raw binary secret blob and returns its generated secret ID
    Create(payload []byte, expireIn time.Duration, reads int) (id string, err error)

    // ReadAndDestroy retrieves and atomically consumes/decrements the raw binary secret blob
    ReadAndDestroy(id string) (payload []byte, readsRemaining int, err error)

    // Count returns total active stored secret entries
    Count() (n int64, err error)

    // Close releases database handles and background pruner channels
    Close() error
}
```

### Storage Engine Concrete Implementation Blueprints

#### 1. SQLite Relational Engine (pkg/storage/sqlite/sqlite.go)
- Schema DDL:
```sql
CREATE TABLE IF NOT EXISTS secrets (
    id TEXT PRIMARY KEY,
    payload BLOB NOT NULL,
    expires_at INTEGER NOT NULL,
    reads_remaining INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_secrets_expires ON secrets(expires_at);
```
- Optimization PRAGMAs:
```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=5000;
PRAGMA temp_store=MEMORY;
PRAGMA cache_size=-64000; -- 64MB Page Cache RAM
```
- Insert Query: INSERT INTO secrets (id, payload, expires_at, reads_remaining) VALUES (?, ?, ?, ?)
- Scan Target: var payload []byte

#### 2. BadgerDB LSM Engine (pkg/storage/badger/badger.go)
- Initialization Options (ZSTD Block Compression):
```go
opts := badger.DefaultOptions(dbPath).WithCompression(options.ZSTD)
db, err := badger.Open(opts)
```
- Write Transaction:
```go
err := db.Update(func(txn *badger.Txn) error {
    entry := badger.NewEntry([]byte(id), payload).WithTTL(expireIn)
    return txn.SetEntry(entry)
})
```

#### 3. Redis Key-Value Store (pkg/storage/redis/redis.go)
- Write Command: rdb.Set(ctx, redisKey(id), payload, expireIn)
- Atomic LUA Retrieval & Burn Script:
```lua
local key = KEYS[1]
local data = redis.call("GET", key)
if not data then return nil end

local reads_rem = redis.call("HINCRBY", key, "r", -1)
if reads_rem <= 0 then
    redis.call("DEL", key)
end
return { data, reads_rem }
```

#### 4. Memcached CAS Engine (pkg/storage/memcached/memcached.go)
- Write Command:
```go
item := &memcache.Item{
    Key:        id,
    Value:      payload, // Raw []byte
    Expiration: int32(expireIn.Seconds()),
}
err := mc.Set(item)
```

---

## 2. Pillar 2: Single-Encoding Attachment Pipeline (~25% Wire Savings)

### The Double-Encoding Problem
In legacy implementations:
1. Client encodes file binary into Base64 string inside SecretAttachment.Data.
2. Outer OTSMeta JSON payload is encrypted and Base64-encoded a second time.
- Legacy Overhead Calculation: 1.333 * 1.333 = 1.777 (+77.7% size inflation).

### Optimized Single-Encoding Protocol (pkg/client/otsMeta.go & src/ots-meta.js)
Store attachment content as raw binary bytes ([]byte / Uint8Array) inside the internal OTSMeta JSON struct before container encryption. Apply Base64 encoding only once on the outer encrypted ciphertext.

```go
type SecretAttachment struct {
    Name    string `json:"name"`
    Type    string `json:"type"`
    Content []byte `json:"data"` // Go JSON marshals []byte into standard Base64 once!
}
```

- Optimized Overhead Calculation: 1.333 (+33.3% standard Base64 size).
- Net Payload Savings: ~25% reduction in network body size for file uploads.

---

## 3. Pillar 3: Unpadded URL-Safe Base64 (RawURLEncoding / RFC 4648 Section 5)

### Standard vs. RawURLEncoding
- Standard Base64 (StdEncoding): Uses +, /, and trailing = padding. Requires URL escaping (%2B, %2F, %3D) when placed in URLs, fragments, or HTTP headers.
- Unpadded Base64URL (RawURLEncoding): Replaces + with -, / with _, and omits trailing = padding characters.

### Go Implementation Guide (pkg/client/crypto.go)
```go
import "encoding/base64"

// EncodeBytesURLSafe encodes raw ciphertext bytes to unpadded URL-safe Base64 string
func EncodeBytesURLSafe(data []byte) string {
    return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeStringURLSafe decodes unpadded URL-safe or standard Base64 string to []byte
func DecodeStringURLSafe(s string) ([]byte, error) {
    // Normalize legacy standard base64 characters if present
    s = strings.ReplaceAll(s, "+", "-")
    s = strings.ReplaceAll(s, "/", "_")
    s = strings.TrimRight(s, "=")
    return base64.RawURLEncoding.DecodeString(s)
}
```

### JS Web SPA Implementation Guide (src/ots-meta.js)
```javascript
export function base64UrlEncode(buffer) {
  const base64 = btoa(String.fromCharCode(...new Uint8Array(buffer)));
  return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64UrlDecode(base64url) {
  let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
  while (base64.length % 4) {
    base64 += '=';
  }
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}
```

---

## 4. Pillar 4: Zero-Allocation Buffer Pooling in Go (sync.Pool)

### High-Throughput Buffer Pool (helpers.go)
To avoid allocating new []byte slices on every HTTP request during high-concurrency API calls:

```go
package main

import (
    "encoding/base64"
    "sync"
)

const maxPooledBufferSize = 2 * 1024 * 1024 // 2MB Max Pooled Buffer

var decodeBufferPool = sync.Pool{
    New: func() any {
        b := make([]byte, 64*1024) // 64KB initial capacity
        return &b
    },
}

// DecodeBase64Pooled decodes src string into a pooled byte slice to minimize GC allocations
func DecodeBase64Pooled(src string) ([]byte, func(), error) {
    reqLen := base64.RawURLEncoding.DecodedLen(len(src))
    
    bufPtr := decodeBufferPool.Get().(*[]byte)
    buf := *bufPtr

    if cap(buf) < reqLen {
        buf = make([]byte, reqLen)
    } else {
        buf = buf[:reqLen]
    }

    n, err := base64.RawURLEncoding.Decode(buf, []byte(src))
    if err != nil {
        // Fallback for legacy padded standard Base64
        n, err = base64.StdEncoding.Decode(buf, []byte(src))
        if err != nil {
            decodeBufferPool.Put(bufPtr)
            return nil, func() {}, err
        }
    }

    cleanup := func() {
        if cap(buf) <= maxPooledBufferSize {
            *bufPtr = buf[:0]
            decodeBufferPool.Put(bufPtr)
        }
    }

    return buf[:n], cleanup, nil
}
```

---

## 5. Pillar 5: Direct Binary Streaming for Large Files (>10 MB)

### Endpoint Blueprint (POST /api/create/raw)
For large file attachments exceeding 10 MB, bypass JSON and Base64 entirely via Content-Type: application/octet-stream:

```go
// handleCreateRaw streams binary body directly into storage backend with 0% encoding overhead
func (a *APIServer) handleCreateRaw(w http.ResponseWriter, r *http.Request) {
    if r.Header.Get("Content-Type") != "application/octet-stream" {
        a.respondError(w, http.StatusBadRequest, "invalid_content_type", "Expected application/octet-stream")
        return
    }

    // Limit reader to configured maximum attachment boundary
    limitedReader := io.LimitReader(r.Body, a.cust.MaxAttachmentSizeTotal+1024)
    rawBytes, err := io.ReadAll(limitedReader)
    if err != nil || int64(len(rawBytes)) > a.cust.MaxAttachmentSizeTotal {
        a.respondError(w, http.StatusBadRequest, "payload_too_large", "Payload exceeds instance storage limit")
        return
    }

    expireIn := a.parseExpiryQuery(r)
    id, err := a.store.Create(rawBytes, expireIn, 1)
    if err != nil {
        a.respondError(w, http.StatusInternalServerError, "storage_error", "Failed to write payload")
        return
    }

    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(map[string]any{
        "success":   true,
        "id":        id,
        "secret_url": fmt.Sprintf("%s/#%s", a.cust.BaseURL, id),
    })
}
```

---

## Performance & Efficiency Matrix

| Metric | Unoptimized Legacy Setup | Optimized Base64 Architecture | Net Gain / Technical Benefit |
| :--- | :--- | :--- | :--- |
| Database Storage Footprint | ASCII Base64 String | Raw []byte / BLOB | 33.3% Storage Savings |
| Attachment Payload Size | Double Base64 (+77.7%) | Single Base64 (+33.3%) | ~25% Wire Size Savings |
| URL Parsing Reliability | Standard Base64 (+, /, =) | RawURLEncoding (-, _) | Zero URL Escaping Failures |
| Memory Allocations | Slice allocated per request | sync.Pool Buffer Reuse | Reduced GC CPU Overhead |
| Large File Uploads (>10 MB) | Base64 JSON String | application/octet-stream | 0% Encoding Overhead |

---

## Implementation Checklist

### Phase 1: Storage Layer []byte Normalization
- [ ] Update pkg/storage/storage.go interface method signatures to []byte.
- [ ] Update pkg/storage/memory/memory.go store.
- [ ] Update pkg/storage/sqlite/sqlite.go schema to BLOB and update SQL queries.
- [ ] Update pkg/storage/badger/badger.go txn.SetEntry.
- [ ] Update pkg/storage/redis/redis.go rdb.Set and LUA script.
- [ ] Update pkg/storage/memcached/memcached.go mc.Set.

### Phase 2: API Handler Base64 Boundary Decoding
- [ ] Update api.go (handleCreate) to decode Base64 body to []byte before calling store.Create().
- [ ] Update api.go (handleRead) to re-encode []byte to Base64 (or RawURLEncoding) before returning JSON.

### Phase 3: Client & SDK Base64URL Conversion
- [ ] Update pkg/client/crypto.go to use base64.RawURLEncoding with fallback decoding.
- [ ] Update src/ots-meta.js to use Base64URL string replacement (.replace(/\+/g, '-').replace(/\//g, '_')).
- [ ] Update pkg/client/otsMeta.go to store raw binary []byte for attachments inside internal struct.

### Phase 4: Verification & Benchmarking
- [ ] Run go test -v ./... across all submodules to verify 100% test pass rate.
- [ ] Run go test -bench=. ./... to verify allocation reduction.
