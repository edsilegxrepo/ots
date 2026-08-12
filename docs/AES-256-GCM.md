# AES-256-GCM Cryptographic Migration Architecture & Security Specification (AES-256-GCM.md)

Target Release: v1.50.0

This document details the security specification, cryptographic parameters, versioning protocol, and technical implementation blueprints for migrating OTS from legacy AES-256-CBC to **AES-256-GCM (AEAD)** with **PBKDF2-HMAC-SHA256**.

---

## Executive Summary

While legacy AES-256-CBC provides confidentiality, CBC mode lacks authenticated encryption with associated data (AEAD). Unauthenticated CBC payloads are susceptible to padding oracle timing attacks, bit-flipping manipulation, and manual HMAC coupling vulnerabilities.

Migrating to AES-256-GCM establishes zero-knowledge AEAD security across the Go client SDK (pkg/client), Vue SPA web client (src/ots-meta.js), and Go CLI tool (cmd/ots-cli).

### Security Gains
1. Built-in 128-bit Authentication Tag: Any single-bit alteration to salt, nonce, ciphertext, or tag causes instant decryption failure before any plaintext processing.
2. Padding Oracle Immunity: GCM operates as a stream-style cipher mode without PKCS#7 padding, eliminating padding oracle timing attacks entirely.
3. AES-NI Acceleration: Utilizes native CPU hardware acceleration on x86-64 and ARM64 processors.
4. Seamless Backward Compatibility: Supports dual-cipher auto-detection to decrypt legacy secrets created under version 1.43.0 and earlier.

---

## 1. Cryptographic Specification & Parameters

| Parameter | Specification | Details & Security Bounds |
| :--- | :--- | :--- |
| Cipher Engine | AES-256-GCM | Authenticated Encryption with Associated Data (AEAD) |
| Key Size | 256 bits (32 bytes) | Cryptographically secure key size |
| Authentication Tag | 128 bits (16 bytes) | Full 16-byte GCM authentication tag |
| Nonce (IV) Size | 96 bits (12 bytes) | NIST SP 800-38D recommended GCM nonce length |
| Key Derivation (KDF) | PBKDF2-HMAC-SHA256 | High-iteration password hashing function |
| PBKDF2 Iterations | 300,000 iterations | NIST recommended iteration threshold |
| Salt Length | 128 bits (16 bytes) | CSPRNG random salt per payload |
| Version Header | `OTSGCM1` (7 bytes) | Magic string prefix for versioned payload routing |

---

## 2. Binary Payload Format

The versioned AES-256-GCM payload structure is self-contained:

```
+------------------+------------------+------------------+-----------------------------------------+
| Header (7 bytes) | Salt (16 bytes)  | Nonce (12 bytes) | Ciphertext + 16-byte Auth Tag (var len) |
|     OTSGCM1      |  PBKDF2 Salt     |    GCM Nonce     |      AES-256-GCM AEAD Encrypted Data    |
+------------------+------------------+------------------+-----------------------------------------+
```

### Total Overhead Breakdown
- Header: 7 bytes (`OTSGCM1`)
- Salt: 16 bytes
- Nonce: 12 bytes
- Auth Tag: 16 bytes
- Total Binary Overhead: Exactly 51 bytes.

---

## 3. Dual-Cipher Decryption Routing Strategy

To ensure zero downtime and 100% backward compatibility for secrets created prior to v1.50.0:

```
Incoming Encrypted Blob
       |
       +---> Starts with "OTSGCM1" (7 bytes)?
       |        |-- YES: Execute AES-256-GCM Decryptor
       |
       +---> Starts with "Salted__" (8 bytes)?
       |        |-- YES: Execute Legacy OpenSSL AES-256-CBC Decryptor
       |
       +---> Unrecognized Header?
                |-- Fallback to Plaintext / Legacy MD5 KDF Parser
```

---

## 4. Go Technical Implementation Blueprint (pkg/client/crypto.go)

```go
package client

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "errors"
    "fmt"
    "io"

    "golang.org/x/crypto/pbkdf2"
)

const (
    gcmHeader       = "OTSGCM1"
    gcmSaltSize     = 16
    gcmNonceSize    = 12
    gcmTagSize      = 16
    gcmKDFIterations = 300000
)

var (
    ErrGCMAuthenticationFailed = errors.New("authentication tag mismatch: payload has been tampered with")
    ErrInvalidGCMPayload       = errors.New("invalid GCM payload size or format")
)

// EncryptGCM encrypts plaintext using AES-256-GCM and PBKDF2-HMAC-SHA256
func EncryptGCM(passphrase string, plaintext []byte) ([]byte, error) {
    salt := make([]byte, gcmSaltSize)
    if _, err := io.ReadFull(rand.Reader, salt); err != nil {
        return nil, fmt.Errorf("generating salt: %w", err)
    }

    key := pbkdf2.Key([]byte(passphrase), salt, gcmKDFIterations, 32, sha256.New)

    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("creating AES cipher: %w", err)
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("creating GCM mode: %w", err)
    }

    nonce := make([]byte, gcmNonceSize)
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, fmt.Errorf("generating nonce: %w", err)
    }

    // Seal appends ciphertext + auth tag to dst
    ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

    // Assemble payload: Header (7B) || Salt (16B) || Nonce (12B) || Ciphertext+Tag
    out := make([]byte, 0, len(gcmHeader)+gcmSaltSize+gcmNonceSize+len(ciphertext))
    out = append(out, []byte(gcmHeader)...)
    out = append(out, salt...)
    out = append(out, nonce...)
    out = append(out, ciphertext...)

    return out, nil
}

// DecryptGCM decrypts payload using AES-256-GCM, verifying authentication tag
func DecryptGCM(passphrase string, payload []byte) ([]byte, error) {
    minSize := len(gcmHeader) + gcmSaltSize + gcmNonceSize + gcmTagSize
    if len(payload) < minSize {
        return nil, ErrInvalidGCMPayload
    }

    header := string(payload[:len(gcmHeader)])
    if header != gcmHeader {
        return nil, fmt.Errorf("invalid GCM header: %s", header)
    }

    offset := len(gcmHeader)
    salt := payload[offset : offset+gcmSaltSize]
    offset += gcmSaltSize

    nonce := payload[offset : offset+gcmNonceSize]
    offset += gcmNonceSize

    ciphertext := payload[offset:]

    key := pbkdf2.Key([]byte(passphrase), salt, gcmKDFIterations, 32, sha256.New)

    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("creating AES cipher: %w", err)
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("creating GCM mode: %w", err)
    }

    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, ErrGCMAuthenticationFailed
    }

    return plaintext, nil
}
```

---

## 5. WebCrypto JavaScript Implementation Blueprint (src/ots-meta.js)

```javascript
const GCM_HEADER = "OTSGCM1";
const GCM_SALT_SIZE = 16;
const GCM_NONCE_SIZE = 12;
const GCM_PBKDF2_ITERATIONS = 300000;

export async function encryptGCM(passphrase, plaintextBytes) {
  const encoder = new TextEncoder();
  const salt = window.crypto.getRandomValues(new Uint8Array(GCM_SALT_SIZE));
  const nonce = window.crypto.getRandomValues(new Uint8Array(GCM_NONCE_SIZE));

  const baseKey = await window.crypto.subtle.importKey(
    "raw",
    encoder.encode(passphrase),
    { name: "PBKDF2" },
    false,
    ["deriveKey"]
  );

  const aesKey = await window.crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt: salt,
      iterations: GCM_PBKDF2_ITERATIONS,
      hash: "SHA-256"
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt"]
  );

  const ciphertext = await window.crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce },
    aesKey,
    plaintextBytes
  );

  const headerBytes = encoder.encode(GCM_HEADER);
  const totalLength = headerBytes.length + salt.length + nonce.length + ciphertext.byteLength;
  const result = new Uint8Array(totalLength);

  let offset = 0;
  result.set(headerBytes, offset); offset += headerBytes.length;
  result.set(salt, offset);        offset += salt.length;
  result.set(nonce, offset);       offset += nonce.length;
  result.set(new Uint8Array(ciphertext), offset);

  return result.buffer;
}

export async function decryptGCM(passphrase, encryptedBuffer) {
  const decoder = new TextDecoder();
  const bytes = new Uint8Array(encryptedBuffer);
  const headerBytes = bytes.subarray(0, 7);
  const header = decoder.decode(headerBytes);

  if (header !== GCM_HEADER) {
    throw new Error("Invalid GCM magic header");
  }

  let offset = 7;
  const salt = bytes.subarray(offset, offset + GCM_SALT_SIZE); offset += GCM_SALT_SIZE;
  const nonce = bytes.subarray(offset, offset + GCM_NONCE_SIZE); offset += GCM_NONCE_SIZE;
  const ciphertext = bytes.subarray(offset);

  const encoder = new TextEncoder();
  const baseKey = await window.crypto.subtle.importKey(
    "raw",
    encoder.encode(passphrase),
    { name: "PBKDF2" },
    false,
    ["deriveKey"]
  );

  const aesKey = await window.crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt: salt,
      iterations: GCM_PBKDF2_ITERATIONS,
      hash: "SHA-256"
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["decrypt"]
  );

  try {
    return await window.crypto.subtle.decrypt(
      { name: "AES-GCM", iv: nonce },
      aesKey,
      ciphertext
    );
  } catch (e) {
    throw new Error("Authentication tag mismatch or invalid passphrase");
  }
}
```

---

## 6. Implementation Action Plan

### Phase 1: Go Crypto Subsystem Implementation
- [ ] Add `EncryptGCM` and `DecryptGCM` to `pkg/client/crypto.go`.
- [ ] Add header router in `pkg/client/otsMeta.go`: if starts with `OTSGCM1`, call `DecryptGCM`; if `Salted__`, call `DecryptOpenSSL`.
- [ ] Add unit tests in `pkg/client/crypto_test.go` verifying GCM encryption, decryption, authentication tag tampering rejection, wrong password rejection, and legacy CBC decryption.

### Phase 2: Web Client SPA Integration
- [ ] Update `src/ots-meta.js` to implement `encryptGCM` and `decryptGCM`.
- [ ] Add magic header sniffing in `src/ots-meta.js` to support dual-cipher decryption in the browser.

### Phase 3: Verification & Integration Testing
- [ ] Run `go test -v ./...` across all submodules to verify 100% test pass rate.
- [ ] Validate cross-interoperability: encrypt with Go CLI, decrypt in Web SPA; encrypt in Web SPA, decrypt with Go CLI.