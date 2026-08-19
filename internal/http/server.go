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

func NewServer(enderecos *service.EnderecoService, pluvial *service.PluvialService, authClient *authsdk.Client) *Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery(), maxBodySizeMiddleware)

	s := &Server{router: router, enderecos: enderecos, pluvial: pluvial, authClient: authClient}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.GET("/openapi.yaml", s.handleOpenAPISpec)
	s.router.GET("/docs", s.handleDocs)
	s.router.GET("/healthz", s.handleHealthz)

	protected := s.router.Group("/")
	protected.Use(RequireAuth(s.authClient))
	{
		protected.POST("/enderecos", s.handleCreateEndereco)
		protected.GET("/enderecos", s.handleListEnderecos)
		protected.GET("/enderecos/:id", s.handleGetEndereco)
		protected.DELETE("/enderecos/:id", s.handleDeleteEndereco)
		protected.POST("/enderecos/:id/pluvial-economy", s.handleCalculatePluvialEconomy)
		protected.GET("/enderecos/:id/pluvial-economy", s.handleGetLatestPluvialEconomy)
	}
}

// maxBodySizeMiddleware caps request bodies at 1MB — Gin has no built-in
// limit.
func maxBodySizeMiddleware(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	c.Next()
}
