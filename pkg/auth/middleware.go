package auth

import (
	"context"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// UserIdentityContextKey is the request context key used to retrieve authenticated UserIdentity.
const UserIdentityContextKey = contextKey("user_identity")

type contextKey string

// AuthMiddleware wraps HTTP handlers to enforce ForwardAuth / Local Basic authentication and RBAC policies.
//
//nolint:revive // AuthMiddleware is explicit and descriptive for callers
type AuthMiddleware struct {
	config      IAMConfig
	forwardAuth *ForwardAuthAuthenticator
	localAuth   *LocalAuthenticator
	rbac        *RBACEvaluator
}

// NewAuthMiddleware initializes authentication middleware based on IAMConfig.
func NewAuthMiddleware(cfg IAMConfig, localAuth *LocalAuthenticator) (*AuthMiddleware, error) {
	am := &AuthMiddleware{
		config:    cfg,
		localAuth: localAuth,
		rbac:      NewRBACEvaluator(cfg.Policy, cfg.ProtectedEndpoints),
	}

	if cfg.Connectors.ForwardAuth.Enabled || cfg.Connector == "forwardauth" {
		fa, err := NewForwardAuthAuthenticator(cfg.Connectors.ForwardAuth)
		if err != nil {
			return nil, err
		}
		am.forwardAuth = fa
	}

	return am, nil
}

// Handler returns HTTP middleware handler.
func (am *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !am.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		if !am.rbac.IsProtectedEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		var identity *UserIdentity
		var authErr error

		// Attempt ForwardAuth header extraction if configured
		if am.forwardAuth != nil {
			identity, authErr = am.forwardAuth.Authenticate(r)
		}

		// Fallback to HTTP Basic Auth if local authenticator is configured
		if identity == nil && am.localAuth != nil {
			identity, authErr = am.localAuth.AuthenticateBasic(r)
		}

		if identity == nil {
			log.WithFields(log.Fields{
				"path":       r.URL.Path,
				"remoteAddr": r.RemoteAddr,
				"error":      authErr,
			}).Warn("Authentication failed on protected endpoint")

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("WWW-Authenticate", `Basic realm="OTS Authentication Required"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
			return
		}

		// Evaluate RBAC authorization policy
		if !am.rbac.IsAuthorized(identity) {
			log.WithFields(log.Fields{
				"username": identity.Username,
				"groups":   identity.Groups,
				"path":     r.URL.Path,
			}).Warn("Authorization denied: user groups do not match allowedGroups policy")

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"Forbidden: insufficient permissions"}`))
			return
		}

		ctx := context.WithValue(r.Context(), UserIdentityContextKey, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIdentity retrieves authenticated UserIdentity from request context.
func GetUserIdentity(r *http.Request) *UserIdentity {
	if identity, ok := r.Context().Value(UserIdentityContextKey).(*UserIdentity); ok {
		return identity
	}
	return nil
}
