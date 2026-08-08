import base64 from "base64-js";

const gcmBanner = new Uint8Array(new TextEncoder().encode("OTSGCM1"));
const opensslBanner = new Uint8Array(new TextEncoder().encode("Salted__"));

const gcmPbkdf2Params = { hash: "SHA-256", iterations: 300000, name: "PBKDF2" };
const cbcPbkdf2Params = { hash: "SHA-512", iterations: 300000, name: "PBKDF2" };

/**
 * Decrypts ciphertext by auto-detecting OTSGCM1 (GCM) vs Salted__ (legacy OpenSSL CBC).
 */
function dec(cipherText: string, passphrase: string): Promise<string> {
  return decrypt(passphrase, cipherText);
}

/**
 * Encrypts plaintext using modern AES-256-GCM with PBKDF2-HMAC-SHA256.
 */
function enc(plainText: string, passphrase: string): Promise<string> {
  return encryptGCM(passphrase, plainText);
}

function decrypt(passphrase: string, encData: string): Promise<string> {
  let data: Uint8Array;
  try {
    data = base64.toByteArray(encData);
  } catch (e) {
    return Promise.reject(new Error("Invalid base64 payload"));
  }

  // Check OTSGCM1 header (7 bytes)
  if (hasPrefix(data, gcmBanner)) {
    return decryptGCM(passphrase, data);
  }

  // Check Salted__ header (8 bytes)
  if (hasPrefix(data, opensslBanner)) {
    return decryptCBC(passphrase, data);
  }

  // Fallback attempt: try GCM then CBC
  return decryptGCM(passphrase, data).catch(() => decryptCBC(passphrase, data));
}

function encryptGCM(passphrase: string, plainData: string): Promise<string> {
  const salt = window.crypto.getRandomValues(new Uint8Array(16));
  const nonce = window.crypto.getRandomValues(new Uint8Array(12));

  return window.crypto.subtle
    .importKey("raw", new TextEncoder().encode(passphrase), "PBKDF2", false, ["deriveKey"])
    .then((baseKey) =>
      window.crypto.subtle.deriveKey(
        { ...gcmPbkdf2Params, salt },
        baseKey,
        { length: 256, name: "AES-GCM" },
        false,
        ["encrypt"],
      ),
    )
    .then((aesKey) =>
      window.crypto.subtle.encrypt(
        { iv: nonce, name: "AES-GCM" },
        aesKey,
        new TextEncoder().encode(plainData),
      ),
    )
    .then((encData) => {
      const banner = gcmBanner;
      const cipherBytes = new Uint8Array(encData);
      const data = new Uint8Array(banner.byteLength + salt.byteLength + nonce.byteLength + cipherBytes.byteLength);

      let offset = 0;
      data.set(banner, offset); offset += banner.byteLength;
      data.set(salt, offset); offset += salt.byteLength;
      data.set(nonce, offset); offset += nonce.byteLength;
      data.set(cipherBytes, offset);

      return base64.fromByteArray(data);
    });
}

function decryptGCM(passphrase: string, data: Uint8Array): Promise<string> {
  if (data.byteLength < 7 + 16 + 12 + 16) {
    return Promise.reject(new Error("Invalid GCM payload size"));
  }

  const salt = data.slice(7, 23);
  const nonce = data.slice(23, 35);
  const ciphertext = data.slice(35);

  return window.crypto.subtle
    .importKey("raw", new TextEncoder().encode(passphrase), "PBKDF2", false, ["deriveKey"])
    .then((baseKey) =>
      window.crypto.subtle.deriveKey(
        { ...gcmPbkdf2Params, salt },
        baseKey,
        { length: 256, name: "AES-GCM" },
        false,
        ["decrypt"],
      ),
    )
    .then((aesKey) =>
      window.crypto.subtle.decrypt(
        { iv: nonce, name: "AES-GCM" },
        aesKey,
        ciphertext,
      ),
    )
    .then((decrypted) => new TextDecoder("utf8").decode(decrypted));
}

function decryptCBC(passphrase: string, data: Uint8Array): Promise<string> {
  const salt = data.slice(8, 16);
  const ciphertext = data.slice(16);

  return deriveKeyCBC(passphrase, salt)
    .then(({ iv, key }) =>
      window.crypto.subtle.decrypt({ iv, name: "AES-CBC" }, key, ciphertext),
    )
    .then((decrypted) => new TextDecoder("utf8").decode(decrypted));
}

function deriveKeyCBC(passphrase: string, salt: Uint8Array): Promise<{ iv: Uint8Array; key: CryptoKey }> {
  return window.crypto.subtle
    .importKey("raw", new TextEncoder().encode(passphrase), "PBKDF2", false, ["deriveBits"])
    .then((passwordKey) =>
      window.crypto.subtle.deriveBits({ ...cbcPbkdf2Params, salt }, passwordKey, 384),
    )
    .then((key) =>
      window.crypto.subtle
        .importKey("raw", key.slice(0, 32), { name: "AES-CBC" }, false, ["encrypt", "decrypt"])
        .then((aesKey) => ({ iv: key.slice(32, 48), key: aesKey })),
    );
}

function hasPrefix(arr: Uint8Array, prefix: Uint8Array): boolean {
  if (arr.byteLength < prefix.byteLength) return false;
  for (let i = 0; i < prefix.byteLength; i++) {
    if (arr[i] !== prefix[i]) return false;
  }
  return true;
}

export default { dec, enc };
