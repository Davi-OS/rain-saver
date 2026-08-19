package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleHealthz(c *gin.Context) {
	writeJSON(c, http.StatusOK, gin.H{"status": "ok"})
}
