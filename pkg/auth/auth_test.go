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
}

func TestRBACEvaluator(t *testing.T) {
	policy := IAMPolicy{
		DefaultPolicy: "deny",
		AllowedGroups: []string{"DevOps", "Security"},
	}

	rbac := NewRBACEvaluator(policy, []string{"/api/create"})

	t.Run("Endpoint Protection Check", func(t *testing.T) {
		assert.True(t, rbac.IsProtectedEndpoint("/api/create"))
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
