package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Davi-OS/go-auth-service/pkg/authsdk"
	"github.com/Davi-OS/rain-saver/internal/service"
)

type Server struct {
	router     *gin.Engine
	enderecos  *service.EnderecoService
	pluvial    *service.PluvialService
	authClient *authsdk.Client
}

// allowedOrigins configures CORS for browser clients (empty disables it).
func NewServer(enderecos *service.EnderecoService, pluvial *service.PluvialService, authClient *authsdk.Client, allowedOrigins []string) *Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery(), corsMiddleware(allowedOrigins), maxBodySizeMiddleware)

	s := &Server{router: router, enderecos: enderecos, pluvial: pluvial, authClient: authClient}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Permissions of the rain-saver system in go-auth-service (catalog
// registered by go-auth-service's cmd/devseed for dev/local).
const (
	PermEnderecoRead     = "endereco:read"
	PermEnderecoWrite    = "endereco:write"
	PermPluvialCalculate = "pluvial:calculate"
)

func (s *Server) routes() {
	s.router.GET("/openapi.yaml", s.handleOpenAPISpec)
	s.router.GET("/docs", s.handleDocs)
	s.router.GET("/healthz", s.handleHealthz)

	protected := s.router.Group("/")
	protected.Use(RequireAuth(s.authClient))
	{
		protected.POST("/enderecos", RequirePermission(PermEnderecoWrite), s.handleCreateEndereco)
		protected.GET("/enderecos", RequirePermission(PermEnderecoRead), s.handleListEnderecos)
		protected.GET("/enderecos/:id", RequirePermission(PermEnderecoRead), s.handleGetEndereco)
		protected.DELETE("/enderecos/:id", RequirePermission(PermEnderecoWrite), s.handleDeleteEndereco)
		protected.POST("/enderecos/:id/pluvial-economy", RequirePermission(PermPluvialCalculate), s.handleCalculatePluvialEconomy)
		protected.GET("/enderecos/:id/pluvial-economy", RequirePermission(PermEnderecoRead), s.handleGetLatestPluvialEconomy)
	}
}

// maxBodySizeMiddleware caps request bodies at 1MB — Gin has no built-in
// limit.
func maxBodySizeMiddleware(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	c.Next()
}
