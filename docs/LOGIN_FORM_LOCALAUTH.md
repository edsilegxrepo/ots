# OTS Local Auth Login Form & Session Subsystem Specification (LOGIN_FORM_LOCALAUTH.md)

**Document Version:** 1.0.0  
**Target Audience:** Frontend Engineers, Backend Architects, Security Engineers  
**Status:** Approved Architectural Specification  
**System:** OTS (One-Time Secret Engine)  

---

## 1. Architectural Vision & User Experience Goals

When OTS is configured in **Enterprise Protected Mode** with **Local Authentication** (`connector: "local"` in `iam.yaml`), secret creation (`POST /api/create`) requires user identification against the local user directory (`users.yaml`). 

To eliminate reliance on browser-native HTTP Basic Auth popups, OTS incorporates a native **Vue 3 Login Form & Session Subsystem**.

```
+-----------------------------------------------------------------------------------+
|                     LOCAL AUTHENTICATION FLOW & UI DECOUPLING                     |
|                                                                                   |
|  1. WHO IS CREATING THE SECRET?                                                   |
|     Authenticated via Native Vue 3 Login Form (users.yaml / Argon2id).            |
|     Establishes HttpOnly Session Cookie or Session Header.                        |
|                                                                                   |
|  2. WHAT IS INSIDE THE SECRET?                                                    |
|     Zero-Knowledge Encryption preserved. AES-256-GCM client-side encryption key     |
|     remains strictly in the browser #URL fragment hash.                           |
|                                                                                   |
|  3. WHO CAN BURN / READ THE SECRET?                                               |
|     100% Anonymous Public Access. GET /secret & GET /api/get/:id bypass auth      |
|     completely so secret recipients never need an account to burn secrets.        |
+-----------------------------------------------------------------------------------+
```

---

## 2. Sequence Diagram & Authentication Pipeline

```mermaid
sequenceDiagram
    autonumber
    actor User as Browser User
    participant App as Vue 3 SPA
    participant API as OTS Backend API
    participant Auth as pkg/auth (Local Argon2id)

    User->>App: Opens OTS Web App
    App->>API: GET /api/settings
    API-->>App: { "authRequired": true, "authProvider": "local" }
    
    App->>API: GET /api/auth/me
    API-->>App: 401 Unauthorized (No active session)
    
    App->>User: Displays Login Modal / Card
    User->>App: Inputs Username & Password
    App->>API: POST /api/auth/login { "username": "bob", "password": "..." }
    
    API->>Auth: Verify Argon2id Hash in users.yaml
    Auth-->>API: Authentication Success (UserIdentity)
    
    API-->>App: 200 OK (Set-Cookie: ots_session=...; HttpOnly; SameSite=Strict)
    App->>User: Unlocks Secret Creation Form & Updates Navbar ("Logged in as bob")
    
    User->>App: Fills Secret Payload & Clicks "Create Secret"
    App->>App: Client-Side AES-256-GCM Encryption
    App->>API: POST /api/create (Includes Session Cookie)
    API-->>App: 200 OK { "secret_id": "a1b2c3d4..." }
```

---

## 3. Backend REST API Endpoint Specification

### Endpoint 1: Query Auth Status (`GET /api/settings`)
Exposes identity requirement metadata to the single-page application on initial page load.

* **Request**: `GET /api/settings`
* **Response `200 OK`**:
```json
{
  "appTitle": "OTS - One-Time Secret Engine",
  "authRequired": true,
  "authProvider": "local",
  "maxAttachmentSizeBytes": 52428800,
  "defaultExpirySeconds": 86400
}
```

---

### Endpoint 2: Authenticate User (`POST /api/auth/login`)
Authenticates local username and password against `users.yaml` (Argon2id).

* **Request**: `POST /api/auth/login`
* **Headers**: `Content-Type: application/json`
* **Body**:
```json
{
  "username": "bob",
  "password": "UserSecurePassword123!"
}
```
* **Response `200 OK`**:
```json
{
  "status": "success",
  "message": "Authenticated successfully",
  "user": {
    "username": "bob",
    "email": "bob@example.com",
    "groups": ["OTS-Creators", "DevOps"],
    "provider": "local"
  }
}
```
* **Set-Cookie Header**:
  `ots_session=<session_token>; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=28800`

* **Response `401 Unauthorized`** (Invalid Credentials):
```json
{
  "error": "Invalid username or password"
}
```

---

### Endpoint 3: Check Current Session (`GET /api/auth/me`)
Returns current authenticated session user details.

* **Request**: `GET /api/auth/me`
* **Response `200 OK`**:
```json
{
  "authenticated": true,
  "username": "bob",
  "email": "bob@example.com",
  "groups": ["OTS-Creators", "DevOps"],
  "provider": "local"
}
```
* **Response `401 Unauthorized`**:
```json
{
  "authenticated": false
}
```

---

### Endpoint 4: Logout User (`POST /api/auth/logout`)
Terminates active user session and clears authentication cookies.

* **Request**: `POST /api/auth/logout`
* **Response `200 OK`**:
```json
{
  "status": "success",
  "message": "Logged out successfully"
}
```
* **Set-Cookie Header**:
  `ots_session=; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=0`

---

## 4. Frontend Vue 3 Component Architecture

### Components Overview
1. **`src/components/login-modal.vue`**:
   - Modern glassmorphism card/modal overlay.
   - Clean forms with username, password, show/hide password toggle, and loading spinners.
   - Inline error banners for invalid credentials or network timeouts.

2. **`src/components/navbar.vue` Integration**:
   - Displays user identity status on the top right navigation bar.
   - Unauthenticated: Displays **"Sign In"** button.
   - Authenticated: Displays user badge (`Logged in as: bob`) with role pills (`DevOps`) and a **"Sign Out"** button.

3. **`src/components/create.vue` Guard**:
   - If `authRequired: true` and user is unauthenticated, secret creation form displays an inline lock screen overlay with a direct button to trigger the Login Modal.
   - If a `POST /api/create` request encounters a `401 Unauthorized` response, automatically intercept and open `login-modal.vue`.

---

## 5. Security & Threat Mitigation Matrix

| Threat Vector | Risk Level | Mitigation Strategy |
| :--- | :--- | :--- |
| **Credential Brute-Forcing** | High | Rate-limit `POST /api/auth/login` by IP (maximum 5 failed attempts per 15 minutes). |
| **Password Hash Re-Hashing** | High | Verifies passwords using OWASP-compliant **Argon2id** (`m=64MB, t=3, p=4`) with constant-time string comparison (`subtle.ConstantTimeCompare`). |
| **Session Hijacking / XSS** | High | Session tokens stored in `HttpOnly`, `SameSite=Strict`, `Secure` HTTP cookies (inaccessible to JavaScript `document.cookie`). |
| **CSRF Attacks** | Medium | `SameSite=Strict` cookie enforcement prevents cross-site request forgery. |
| **Zero-Knowledge Compromise** | Critical | Server authentication strictly controls access to `POST /api/create`. Decryption keys (`#` fragment) remain client-side only and are never exposed to auth endpoints. |
