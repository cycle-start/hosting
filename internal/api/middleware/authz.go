package middleware

import (
	"context"
	"net/http"

	"github.com/edvin/hosting/internal/api/response"
)

// GetIdentity extracts the APIKeyIdentity from the request context.
func GetIdentity(ctx context.Context) *APIKeyIdentity {
	identity, _ := ctx.Value(APIKeyIdentityKey).(*APIKeyIdentity)
	return identity
}

// HasScope checks if the identity has the given resource:action scope (or the *:* wildcard).
func HasScope(identity *APIKeyIdentity, resource, action string) bool {
	if identity == nil {
		return false
	}
	target := resource + ":" + action
	for _, s := range identity.Scopes {
		if s == "*:*" || s == target {
			return true
		}
	}
	return false
}

// HasBrandAccess checks if the identity can access the given brand ID.
func HasBrandAccess(identity *APIKeyIdentity, brandID string) bool {
	if identity == nil {
		return false
	}
	for _, b := range identity.Brands {
		if b == "*" || b == brandID {
			return true
		}
	}
	return false
}

// IsPlatformAdmin checks if the identity has wildcard brand access.
func IsPlatformAdmin(identity *APIKeyIdentity) bool {
	if identity == nil {
		return false
	}
	for _, b := range identity.Brands {
		if b == "*" {
			return true
		}
	}
	return false
}

// RequireScope returns middleware that checks the key has the given resource:action scope.
func RequireScope(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := GetIdentity(r.Context())
			if !HasScope(identity, resource, action) {
				response.WriteError(w, http.StatusForbidden, "insufficient scope: requires "+resource+":"+action)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePlatformAdmin returns middleware that checks the key has wildcard brand access.
func RequirePlatformAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := GetIdentity(r.Context())
			if !IsPlatformAdmin(identity) {
				response.WriteError(w, http.StatusForbidden, "platform admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BrandIDs returns the identity's brand list for use in query filtering.
// Returns nil if the identity is a platform admin (wildcard).
func BrandIDs(ctx context.Context) []string {
	identity := GetIdentity(ctx)
	if identity == nil {
		return nil
	}
	for _, b := range identity.Brands {
		if b == "*" {
			return nil
		}
	}
	return identity.Brands
}
