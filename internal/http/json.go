package http

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const maxRequestBodyBytes = 1 << 20 // 1MB

func init() {
	binding.EnableDecoderDisallowUnknownFields = true
}

type errorResponse struct {
	Error string `json:"error"`
}

func decodeJSON(c *gin.Context, dst any) error {
	return c.ShouldBindJSON(dst)
}

func writeJSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

func writeError(c *gin.Context, status int, message string) {
	writeJSON(c, status, errorResponse{Error: message})
}
