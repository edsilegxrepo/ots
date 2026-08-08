// Package auth implements authentication and authorization test suites.
package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardAuthAuthenticator(t *testing.T) {
	cfg := ForwardAuthConfig{
		Enabled:         true,
		UserHeader:      "Remote-User",
		EmailHeader:     "Remote-Email",
		GroupsHeader:    "Remote-Groups",
		HeaderDelimiter: ",",
		TrustedProxies:  []string{"127.0.0.1", "10.0.0.0/8"},
	}

	fa, err := NewForwardAuthAuthenticator(cfg)
	require.NoError(t, err)

	t.Run("Trusted Proxy Header Extraction", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("Remote-User", "alice@company.com")
		req.Header.Set("Remote-Email", "alice@company.com")
		req.Header.Set("Remote-Groups", "DevOps, SecOps")

		identity, err := fa.Authenticate(req)
		require.NoError(t, err)
		assert.Equal(t, "alice@company.com", identity.Username)
		assert.Equal(t, "alice@company.com", identity.Email)
		assert.Equal(t, []string{"DevOps", "SecOps"}, identity.Groups)
		assert.Equal(t, "forwardauth", identity.Provider)
	})

	t.Run("Untrusted Proxy Spoofing Rejection", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "203.0.113.195:12345" // Untrusted IP
		req.Header.Set("Remote-User", "hacker@evil.com")

		identity, err := fa.Authenticate(req)
		require.ErrorIs(t, err, ErrUntrustedProxy)
		assert.Nil(t, identity)
	})

	t.Run("Missing User Header Rejection", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:54321"

		identity, err := fa.Authenticate(req)
		require.ErrorIs(t, err, ErrMissingUser)
		assert.Nil(t, identity)
	})
}

func TestLocalArgon2idAuthenticator(t *testing.T) {
	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.yaml")

	passHash, err := HashPassword("SuperSecret123!")
	require.NoError(t, err)
	assert.True(t, VerifyPassword("SuperSecret123!", passHash))
	assert.False(t, VerifyPassword("WrongPass", passHash))

	la, err := NewLocalAuthenticator(usersFile)
	require.NoError(t, err)

	err = la.AddUser(UserRecord{
		Username: "bob",
		Hash:     passHash,
		Email:    "bob@company.com",
		Groups:   []string{"OTS-Creators"},
	})
	require.NoError(t, err)

	err = la.SaveUsers()
	require.NoError(t, err)

	t.Run("Valid HTTP Basic Credentials", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.SetBasicAuth("bob", "SuperSecret123!")

		identity, err := la.AuthenticateBasic(req)
		require.NoError(t, err)
		assert.Equal(t, "bob", identity.Username)
		assert.Equal(t, []string{"OTS-Creators"}, identity.Groups)
		assert.Equal(t, "local", identity.Provider)
	})

	t.Run("Invalid HTTP Basic Credentials", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.SetBasicAuth("bob", "WrongPass")

		identity, err := la.AuthenticateBasic(req)
		require.ErrorIs(t, err, ErrInvalidCredentials)
		assert.Nil(t, identity)
	})

	t.Run("User Management Operations", func(t *testing.T) {
		// Add disabled user
		err := la.AddUser(UserRecord{
			Username: "disabled_user",
			Hash:     passHash,
			Disabled: true,
		})
		require.NoError(t, err)

		// Update existing user
		err = la.AddUser(UserRecord{
			Username: "disabled_user",
			Hash:     passHash,
			Disabled: true,
		})
		require.NoError(t, err)

		// List users
		users := la.ListUsers()
		require.Len(t, users, 2)

		// Authenticate disabled user returns ErrUserDisabled
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.SetBasicAuth("disabled_user", "SuperSecret123!")
		_, err = la.AuthenticateBasic(req)
		require.ErrorIs(t, err, ErrUserDisabled)

		// Authenticate non-existent user returns ErrUserNotFound
		req.SetBasicAuth("unknown_user", "SuperSecret123!")
		_, err = la.AuthenticateBasic(req)
		require.ErrorIs(t, err, ErrUserNotFound)

		// Delete user
		err = la.DeleteUser("bob")
		require.NoError(t, err)

		// SaveUsers error on empty path
		emptyLA := &LocalAuthenticator{}
		require.Error(t, emptyLA.SaveUsers())
		require.NoError(t, emptyLA.loadUsersLocked())
	})

	t.Run("Argon2id Verification Failures", func(t *testing.T) {
		assert.False(t, VerifyPassword("any", "invalid_argon_string"))
		assert.False(t, VerifyPassword("any", "$argon2id$v=19$m=65536,t=3,p=2$invalid_b64_salt$invalid_b64_hash"))
	})
}

func TestRBACEvaluator(t *testing.T) {
	policy := IAMPolicy{
		DefaultPolicy: "deny",
		AllowedGroups: []string{"DevOps", "Security"},
	}

	rbac := NewRBACEvaluator(policy, []string{"/api/create"})

	t.Run("Endpoint Protection Check", func(t *testing.T) {
		assert.True(t, rbac.IsProtectedEndpoint("/api/create"))
		assert.True(t, rbac.IsProtectedEndpoint("/api/create/raw"), "/api/create/raw must be protected")
		assert.True(t, rbac.IsProtectedEndpoint("/api/create/"), "trailing slash must be normalized")
		assert.False(t, rbac.IsProtectedEndpoint("/secret"))
		assert.False(t, rbac.IsProtectedEndpoint("/api/get/12345"))
		assert.False(t, rbac.IsProtectedEndpoint("/api/healthz"))
	})

	t.Run("Authorization Evaluation", func(t *testing.T) {
		authorizedUser := &UserIdentity{Username: "alice", Groups: []string{"DevOps"}}
		unauthorizedUser := &UserIdentity{Username: "charlie", Groups: []string{"Guest"}}

		assert.True(t, rbac.IsAuthorized(authorizedUser))
		assert.False(t, rbac.IsAuthorized(unauthorizedUser))
	})
}

func TestLoadIAMConfig(t *testing.T) {
	yamlWithIAMRoot := []byte(`
iam:
  enabled: true
  connector: forwardauth
  policy:
    allowedGroups: ["DevOps"]
`)

	cfg, err := LoadIAMConfig(yamlWithIAMRoot)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "forwardauth", cfg.Connector)
	assert.Equal(t, []string{"DevOps"}, cfg.Policy.AllowedGroups)
}

func TestAuthMiddleware(t *testing.T) {
	cfg := IAMConfig{
		Enabled:            true,
		Connector:          "forwardauth",
		ProtectedEndpoints: []string{"/api/create"},
		Policy: IAMPolicy{
			DefaultPolicy: "deny",
			AllowedGroups: []string{"DevOps"},
		},
		Connectors: IAMConnectors{
			ForwardAuth: ForwardAuthConfig{
				Enabled:         true,
				UserHeader:      "Remote-User",
				GroupsHeader:    "Remote-Groups",
				HeaderDelimiter: ",",
				TrustedProxies:  []string{"127.0.0.1"},
			},
		},
	}

	am, err := NewAuthMiddleware(cfg, nil)
	require.NoError(t, err)

	handler := am.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetUserIdentity(r)
		if id != nil {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(id.Username))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("anonymous"))
		}
	}))

	t.Run("Unprotected Endpoint Pass-Through", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "anonymous", rec.Body.String())
	})

	t.Run("Protected Authorized Request", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("Remote-User", "alice")
		req.Header.Set("Remote-Groups", "DevOps")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "alice", rec.Body.String())
	})

	t.Run("Protected Unauthenticated Request", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Protected Forbidden Request", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/create", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("Remote-User", "charlie")
		req.Header.Set("Remote-Groups", "Guests")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestAuthExtendedConnectorAndPolicyValidation(t *testing.T) {
	// Test LoadIAMConfig invalid YAML
	_, err := LoadIAMConfig([]byte("invalid_yaml: ["))
	require.Error(t, err)

	// Test NewAuthMiddleware disabled IAM
	cfgDisabled := IAMConfig{Enabled: false}
	amDisabled, err := NewAuthMiddleware(cfgDisabled, nil)
	require.NoError(t, err)
	assert.False(t, amDisabled.config.Enabled)

	// Test RBAC Default Policy Allow
	var emptyGroups []string
	policyAllow := IAMPolicy{
		DefaultPolicy: "allow",
		AllowedGroups: emptyGroups,
	}
	eval := NewRBACEvaluator(policyAllow, []string{"/api/create"})
	id := &UserIdentity{Username: "bob", Groups: []string{"Users"}}
	assert.True(t, eval.IsAuthorized(id))

	// Test RBAC Nil User
	assert.False(t, eval.IsAuthorized(nil))
}
