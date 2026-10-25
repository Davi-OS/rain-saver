package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newCORSTestRouter(allowed []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(corsMiddleware(allowed))
	// Stands in for an auth-protected route: preflights must never reach it.
	r.GET("/protected", func(c *gin.Context) { c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"}) })
	return r
}

func TestCORS(t *testing.T) {
	const app = "http://localhost:5173"
	r := newCORSTestRouter([]string{app})

	do := func(method, origin, reqMethod string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/protected", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if reqMethod != "" {
			req.Header.Set("Access-Control-Request-Method", reqMethod)
			req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("preflight from allowed origin", func(t *testing.T) {
		w := do(http.MethodOptions, app, http.MethodPost)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != app {
			t.Errorf("Allow-Origin = %q, want %q", got, app)
		}
		if got := w.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
			t.Errorf("Allow-Headers = %q", got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("Allow-Credentials = %q, want unset", got)
		}
	})

	t.Run("preflight from other origin is refused", func(t *testing.T) {
		w := do(http.MethodOptions, "https://evil.example", http.MethodPost)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want unset", got)
		}
	})

	t.Run("actual request keeps CORS headers on error responses", func(t *testing.T) {
		w := do(http.MethodGet, app, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want the handler's 401", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != app {
			t.Errorf("Allow-Origin = %q, want %q", got, app)
		}
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("Vary = %q, want Origin", got)
		}
	})

	t.Run("same-origin / non-browser request untouched", func(t *testing.T) {
		w := do(http.MethodGet, "", "")
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want unset", got)
		}
	})

	t.Run("empty allowlist disables CORS", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/protected", nil)
		req.Header.Set("Origin", app)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		newCORSTestRouter(nil).ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want unset", got)
		}
	})
}
