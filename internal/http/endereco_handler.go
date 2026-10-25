package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/service"
)

// Coordinates are optional: without them the address is geocoded from
// logradouro/number/cidade/estado, so cidade and estado become required.
type enderecoCreateRequest struct {
	Logradouro string   `json:"logradouro" binding:"required"`
	Number     string   `json:"number" binding:"required"`
	Bairro     string   `json:"bairro"`
	Cep        string   `json:"cep"`
	Latitude   *float64 `json:"latitude" binding:"omitempty,gte=-90,lte=90"`
	Longitude  *float64 `json:"longitude" binding:"omitempty,gte=-180,lte=180"`
	Cidade     string   `json:"cidade"`
	Estado     string   `json:"estado"`
}

func (r enderecoCreateRequest) validate() string {
	if (r.Latitude == nil) != (r.Longitude == nil) {
		return "latitude and longitude must be sent together"
	}
	if r.Latitude == nil && (strings.TrimSpace(r.Cidade) == "" || strings.TrimSpace(r.Estado) == "") {
		return "cidade and estado are required when latitude/longitude are not sent"
	}
	return ""
}

type enderecoResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Logradouro string    `json:"logradouro"`
	Number     string    `json:"number"`
	Bairro     string    `json:"bairro"`
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
		Bairro:     e.Bairro,
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
	if msg := req.validate(); msg != "" {
		writeError(c, http.StatusBadRequest, msg)
		return
	}

	e, err := s.enderecos.Create(c.Request.Context(), service.CreateEnderecoInput{
		UserID:     userID,
		Logradouro: req.Logradouro,
		Number:     req.Number,
		Bairro:     req.Bairro,
		Cep:        req.Cep,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		Cidade:     req.Cidade,
		Estado:     req.Estado,
	})
	switch {
	case errors.Is(err, service.ErrAddressNotFound):
		writeError(c, http.StatusUnprocessableEntity, "address could not be located")
		return
	case errors.Is(err, service.ErrGeocodingUnavailable):
		writeError(c, http.StatusServiceUnavailable, "geocoding unavailable, try again later")
		return
	case err != nil:
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
