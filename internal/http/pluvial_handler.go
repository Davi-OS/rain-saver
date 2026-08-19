package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
	"github.com/Davi-OS/rain-saver/internal/service"
)

type pluvialCalculationRequest struct {
	AreaM2 float64 `json:"area_m2" binding:"required,gt=0"`
}

type monthlyCaptationResponse struct {
	Year           int     `json:"year"`
	Month          int     `json:"month"`
	RainfallMM     float64 `json:"rainfall_mm"`
	CapturedLiters float64 `json:"captured_liters"`
}

type pluvialEconomyResponse struct {
	ID               string                     `json:"id"`
	EnderecoID       string                     `json:"endereco_id"`
	AreaM2           float64                    `json:"area_m2"`
	PeriodStart      time.Time                  `json:"period_start"`
	PeriodEnd        time.Time                  `json:"period_end"`
	CreatedAt        time.Time                  `json:"created_at"`
	MonthlyCaptation []monthlyCaptationResponse `json:"monthly_captation"`
}

func pluvialEconomyToResponse(pe *domain.PluvialEconomy) pluvialEconomyResponse {
	months := make([]monthlyCaptationResponse, len(pe.MonthlyCaptation))
	for i, m := range pe.MonthlyCaptation {
		months[i] = monthlyCaptationResponse{
			Year:           m.Year,
			Month:          m.Month,
			RainfallMM:     m.RainfallMM,
			CapturedLiters: m.CapturedLiters,
		}
	}
	return pluvialEconomyResponse{
		ID:               pe.ID,
		EnderecoID:       pe.EnderecoID,
		AreaM2:           pe.AreaM2,
		PeriodStart:      pe.PeriodStart,
		PeriodEnd:        pe.PeriodEnd,
		CreatedAt:        pe.CreatedAt,
		MonthlyCaptation: months,
	}
}

func (s *Server) handleCalculatePluvialEconomy(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req pluvialCalculationRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	pe, err := s.pluvial.Calculate(c.Request.Context(), c.Param("id"), userID, req.AreaM2)
	if errors.Is(err, service.ErrEnderecoNotFound) {
		writeError(c, http.StatusNotFound, "endereco not found")
		return
	}
	if errors.Is(err, service.ErrWeatherProvider) {
		writeError(c, http.StatusBadGateway, "weather provider error")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusCreated, pluvialEconomyToResponse(pe))
}

func (s *Server) handleGetLatestPluvialEconomy(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	pe, err := s.pluvial.Latest(c.Request.Context(), c.Param("id"), userID)
	if errors.Is(err, service.ErrEnderecoNotFound) || errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "no pluvial economy calculated for this endereco")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, pluvialEconomyToResponse(pe))
}
