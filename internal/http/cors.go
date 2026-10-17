package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// corsMiddleware lets browser apps served from allowedOrigins call this API.
// Tokens travel in the Authorization header, never in cookies, so
// credentialed CORS is not enabled. An empty list disables CORS entirely:
// no headers are sent and browsers block cross-origin calls.
//
// It must run before any auth middleware: preflight requests carry no token
// and are answered here, and error responses (e.g. 401) must still carry
// Access-Control-Allow-Origin so the browser app can read them.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		h := c.Writer.Header()
		h.Add("Vary", "Origin")
		preflight := c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != ""

		if _, ok := allowed[origin]; ok {
			h.Set("Access-Control-Allow-Origin", origin)
			if preflight {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Max-Age", "600")
			}
		}
		if preflight {
			// Disallowed origins get the same 204 without allow headers,
			// which the browser treats as a refusal.
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
