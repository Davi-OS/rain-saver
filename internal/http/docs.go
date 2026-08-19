package http

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openAPISpec []byte

const swaggerUIPage = `<!DOCTYPE html>
<html>
<head>
  <title>rain-saver API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/openapi.yaml',
        dom_id: '#swagger-ui',
      });
    };
  </script>
</body>
</html>
`

func (s *Server) handleOpenAPISpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml", openAPISpec)
}

func (s *Server) handleDocs(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUIPage))
}
