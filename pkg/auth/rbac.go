package auth

import (
	"path"
	"strings"
)

func cleanPath(p string) string {
	c := path.Clean(strings.TrimSpace(p))
	if c == "." {
		return "/"
	}
	return c
}

// RBACEvaluator handles group-based authorization checks against IAMPolicy.
type RBACEvaluator struct {
	policy             IAMPolicy
	protectedEndpoints map[string]bool
}

// NewRBACEvaluator creates a new RBAC policy evaluator.
func NewRBACEvaluator(policy IAMPolicy, protectedEndpoints []string) *RBACEvaluator {
	peMap := make(map[string]bool)
	for _, ep := range protectedEndpoints {
		c := cleanPath(ep)
		if c != "" {
			peMap[c] = true
		}
	}

	return &RBACEvaluator{
		policy:             policy,
		protectedEndpoints: peMap,
	}
}

// IsProtectedEndpoint returns true if the requested HTTP path requires authentication.
func (r *RBACEvaluator) IsProtectedEndpoint(reqPath string) bool {
	reqPath = cleanPath(reqPath)

	// Redemption endpoints are strictly public & anonymous
	if reqPath == "/secret" || strings.HasPrefix(reqPath, "/api/get/") || reqPath == "/api/healthz" || reqPath == "/api/isWritable" || reqPath == "/api/settings" {
		return false
	}

	if r.protectedEndpoints[reqPath] {
		return true
	}

	// Default protection for secret creation
	if reqPath == "/api/create" {
		return true
	}

	return false
}

// IsAuthorized checks if the identity is permitted under IAMPolicy.AllowedGroups.
func (r *RBACEvaluator) IsAuthorized(user *UserIdentity) bool {
	if user == nil {
		return false
	}

	if len(r.policy.AllowedGroups) == 0 {
		// If no allowedGroups specified, default to allow if authenticated
		return r.policy.DefaultPolicy != "deny"
	}

	for _, userGroup := range user.Groups {
		for _, allowedGroup := range r.policy.AllowedGroups {
			if strings.EqualFold(userGroup, allowedGroup) {
				return true
			}
		}
	}

	return false
}

// IsFeatureAllowed checks if the identity is permitted for a specific feature policy (e.g., allowLargeAttachments).
func (r *RBACEvaluator) IsFeatureAllowed(user *UserIdentity, featureName string) (bool, int64) {
	if user == nil || r.policy.FeaturePolicies == nil {
		return false, 0
	}

	feat, exists := r.policy.FeaturePolicies[featureName]
	if !exists {
		return false, 0
	}

	for _, userGroup := range user.Groups {
		for _, allowedGroup := range feat.AllowedGroups {
			if strings.EqualFold(userGroup, allowedGroup) {
				return true, feat.MaxSizeBytes
			}
		}
	}

	return false, 0
}
