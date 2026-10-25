package http_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"

	"github.com/Davi-OS/go-auth-sdk/authsdk"
	"github.com/Davi-OS/go-auth-sdk/token"
	rainhttp "github.com/Davi-OS/rain-saver/internal/http"
)

// A fake auth server: the JWKS of the test key and an empty revocation
// delta, the two endpoints authsdk.New needs. Tokens are signed in signToken
// the way go-auth-service signs them.
func newFakeAuthServer(kid string, pub *ecdsa.PublicKey) *httptest.Server {
	jwks := token.JWKS{Keys: []token.JWK{token.PublicKeyToJWK(kid, pub)}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	})
	mux.HandleFunc("GET /revocations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	return httptest.NewServer(mux)
}

func signToken(t *testing.T, priv *ecdsa.PrivateKey, kid, subject, audience string, perms ...string) string {
	t.Helper()
	now := time.Now()
	claims := authsdk.Claims{
		Permissions: perms,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Subject:   subject,
			Audience:  jwtlib.ClaimStrings{audience},
			Issuer:    token.DefaultIssuer,
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Minute)),
			ID:        "test-jti",
		},
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodES256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func newTestRouter(client *authsdk.Client) http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", rainhttp.RequireAuth(client), func(c *gin.Context) {
		userID, _ := rainhttp.UserIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})
	r.GET("/needs-write", rainhttp.RequireAuth(client), rainhttp.RequirePermission(rainhttp.PermEnderecoWrite), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return r
}

func doWithBearer(url, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

func TestRequireAuth(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const kid = "test-key"
	fake := newFakeAuthServer(kid, &priv.PublicKey)
	defer fake.Close()

	client, err := authsdk.New(context.Background(), authsdk.Config{
		BaseURL:      fake.URL,
		Audience:     "rain-saver",
		PollInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("authsdk.New: %v", err)
	}
	defer client.Close()

	ts := httptest.NewServer(newTestRouter(client))
	defer ts.Close()

	t.Run("missing token", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/protected")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		resp, err := doWithBearer(ts.URL+"/protected", "not-a-jwt")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("wrong audience", func(t *testing.T) {
		token := signToken(t, priv, kid, "user-1", "some-other-system")
		resp, err := doWithBearer(ts.URL+"/protected", token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		token := signToken(t, priv, kid, "user-1", "rain-saver")
		resp, err := doWithBearer(ts.URL+"/protected", token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		var body map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body["user_id"] != "user-1" {
			t.Errorf("user_id = %q, want user-1", body["user_id"])
		}
	})
}

func TestRequirePermission(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const kid = "test-key"
	fake := newFakeAuthServer(kid, &priv.PublicKey)
	defer fake.Close()
	client, err := authsdk.New(context.Background(), authsdk.Config{BaseURL: fake.URL, Audience: "rain-saver", PollInterval: time.Second})
	if err != nil {
		t.Fatalf("authsdk.New: %v", err)
	}
	defer client.Close()
	ts := httptest.NewServer(newTestRouter(client))
	defer ts.Close()

	tests := []struct {
		name   string
		perms  []string
		status int
	}{
		{"no permissions (account without access to rain-saver)", nil, http.StatusForbidden},
		{"other permissions only", []string{rainhttp.PermEnderecoRead}, http.StatusForbidden},
		{"required permission", []string{rainhttp.PermEnderecoRead, rainhttp.PermEnderecoWrite}, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := doWithBearer(ts.URL+"/needs-write", signToken(t, priv, kid, "user-1", "rain-saver", tt.perms...))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.status)
			}
		})
	}
}
