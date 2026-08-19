package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/service"
)

type enderecoCreateRequest struct {
	Logradouro string  `json:"logradouro" binding:"required"`
	Number     string  `json:"number" binding:"required"`
	Cep        string  `json:"cep"`
	Latitude   float64 `json:"latitude" binding:"required"`
	Longitude  float64 `json:"longitude" binding:"required"`
	Cidade     string  `json:"cidade"`
	Estado     string  `json:"estado"`
}

type enderecoResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Logradouro string    `json:"logradouro"`
	Number     string    `json:"number"`
	Cep        string    `json:"cep"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Cidade     string    `json:"cidade"`
	Estado     string    `json:"estado"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func enderecoToResponse(e *domain.Endereco) enderecoResponse {
	return enderecoResponse{
		ID:         e.ID,
		UserID:     e.UserID,
		Logradouro: e.Logradouro,
		Number:     e.Number,
		Cep:        e.Cep,
		Latitude:   e.Latitude,
		Longitude:  e.Longitude,
		Cidade:     e.Cidade,
		Estado:     e.Estado,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}

func (s *Server) handleCreateEndereco(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req enderecoCreateRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	e, err := s.enderecos.Create(c.Request.Context(), service.CreateEnderecoInput{
		UserID:     userID,
		Logradouro: req.Logradouro,
		Number:     req.Number,
		Cep:        req.Cep,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		Cidade:     req.Cidade,
		Estado:     req.Estado,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusCreated, enderecoToResponse(e))
}

func (s *Server) handleListEnderecos(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	list, err := s.enderecos.List(c.Request.Context(), userID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]enderecoResponse, len(list))
	for i, e := range list {
		out[i] = enderecoToResponse(&e)
	}
	writeJSON(c, http.StatusOK, out)
}

func (s *Server) handleGetEndereco(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	e, err := s.enderecos.Get(c.Request.Context(), c.Param("id"), userID)
	if errors.Is(err, service.ErrEnderecoNotFound) {
		writeError(c, http.StatusNotFound, "endereco not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, enderecoToResponse(e))
}

func (s *Server) handleDeleteEndereco(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := s.enderecos.Delete(c.Request.Context(), c.Param("id"), userID)
	if errors.Is(err, service.ErrEnderecoNotFound) {
		writeError(c, http.StatusNotFound, "endereco not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.Status(http.StatusNoContent)
}
