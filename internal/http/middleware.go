package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Davi-OS/go-auth-sdk/authsdk"
)

const claimsContextKey = "rainsaver_claims"

// RequireAuth verifies the bearer access token against the auth service
// (signature, issuer, audience="rain-saver", revocation) via authsdk.Client
// and stores the resulting claims in the Gin context. authsdk's own
// middleware (Authenticate/RequirePermission/RequireRole) is written for
// net/http (func(http.Handler) http.Handler); Gin route groups need
// gin.HandlerFunc, so this calls Client.Verify directly instead of wrapping
// the http.Handler-shaped middleware — mirroring what authsdk's private
// authenticateRequest does internally.
func RequireAuth(client *authsdk.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			writeError(c, http.StatusUnauthorized, "unauthorized: "+authsdk.ErrMissingToken.Error())
			c.Abort()
			return
		}

		claims, err := client.Verify(token)
		if err != nil {
			writeError(c, http.StatusUnauthorized, "unauthorized: "+err.Error())
			c.Abort()
			return
		}

		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

// RequirePermission rejects the request with 403 unless the token carries
// perm. Must run after RequireAuth. Tokens of accounts without access to
// rain-saver (no organization enabled for it) carry no permissions at all,
// so this is also what keeps them out.
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, _ := c.Get(claimsContextKey)
		claims, ok := v.(*authsdk.Claims)
		if !ok || !claims.HasPermission(perm) {
			writeError(c, http.StatusForbidden, "forbidden: missing permission "+perm)
			c.Abort()
			return
		}
		c.Next()
	}
}

// UserIDFromContext extracts the authenticated user's id (the JWT sub
// claim) set by RequireAuth. Handlers must call this rather than trust any
// client-supplied user id field.
func UserIDFromContext(c *gin.Context) (string, bool) {
	v, ok := c.Get(claimsContextKey)
	if !ok {
		return "", false
	}
	claims, ok := v.(*authsdk.Claims)
	if !ok {
		return "", false
	}
	return claims.Subject, true
}

func bearerToken(c *gin.Context) string {
	const prefix = "Bearer "
	h := c.GetHeader("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}
