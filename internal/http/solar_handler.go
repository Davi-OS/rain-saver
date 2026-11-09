package http

import (
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
	"github.com/Davi-OS/rain-saver/internal/service"
)

type solarCalculationRequest struct {
	PanelPowerW float64 `json:"panel_power_w" binding:"required,gt=0,lte=1000"`
	PanelCount  int     `json:"panel_count" binding:"required,gte=1,lte=1000"`
	// TariffBRLPerKWh is optional; savings are only reported when it is set.
	TariffBRLPerKWh *float64 `json:"tariff_brl_per_kwh" binding:"omitempty,gt=0,lte=10"`
}

type monthlyGenerationResponse struct {
	Year          int      `json:"year"`
	Month         int      `json:"month"`
	AvgDailyKWhM2 float64  `json:"avg_daily_kwh_m2"`
	GeneratedKWh  float64  `json:"generated_kwh"`
	SavingsBRL    *float64 `json:"savings_brl,omitempty"`
}

type solarEconomyResponse struct {
	ID                   string                      `json:"id"`
	EnderecoID           string                      `json:"endereco_id"`
	PanelPowerW          float64                     `json:"panel_power_w"`
	PanelCount           int                         `json:"panel_count"`
	SystemKWp            float64                     `json:"system_kwp"`
	LossFactor           float64                     `json:"loss_factor"`
	PeriodStart          time.Time                   `json:"period_start"`
	PeriodEnd            time.Time                   `json:"period_end"`
	CreatedAt            time.Time                   `json:"created_at"`
	StationCode          string                      `json:"station_code"`
	DistanceKm           float64                     `json:"distance_km"`
	TariffBRLPerKWh      *float64                    `json:"tariff_brl_per_kwh,omitempty"`
	TotalKWh             float64                     `json:"total_kwh"`
	MonthlyAvgKWh        float64                     `json:"monthly_avg_kwh"`
	TotalSavingsBRL      *float64                    `json:"total_savings_brl,omitempty"`
	MonthlyAvgSavingsBRL *float64                    `json:"monthly_avg_savings_brl,omitempty"`
	MonthlyGeneration    []monthlyGenerationResponse `json:"monthly_generation"`
}

func solarEconomyToResponse(se *domain.SolarEconomy) solarEconomyResponse {
	months := make([]monthlyGenerationResponse, len(se.MonthlyGeneration))
	for i, m := range se.MonthlyGeneration {
		months[i] = monthlyGenerationResponse{
			Year:          m.Year,
			Month:         m.Month,
			AvgDailyKWhM2: m.AvgDailyKWhM2,
			GeneratedKWh:  m.GeneratedKWh,
		}
		if savings, ok := se.MonthSavingsBRL(m); ok {
			months[i].SavingsBRL = &savings
		}
	}
	total := se.TotalKWh()
	var monthlyAvg float64
	if len(months) > 0 {
		monthlyAvg = total / float64(len(months))
	}
	var totalSavings, avgSavings *float64
	if savings, ok := se.TotalSavingsBRL(); ok {
		totalSavings = &savings
		if len(months) > 0 {
			avg := math.Round(savings/float64(len(months))*100) / 100
			avgSavings = &avg
		}
	}
	return solarEconomyResponse{
		ID:                   se.ID,
		EnderecoID:           se.EnderecoID,
		PanelPowerW:          se.PanelPowerW,
		PanelCount:           se.PanelCount,
		SystemKWp:            se.SystemKWp,
		LossFactor:           se.LossFactor,
		PeriodStart:          se.PeriodStart,
		PeriodEnd:            se.PeriodEnd,
		CreatedAt:            se.CreatedAt,
		StationCode:          se.StationCode,
		DistanceKm:           se.DistanceKm,
		TariffBRLPerKWh:      se.TariffBRLPerKWh,
		TotalKWh:             math.Round(total*100) / 100,
		MonthlyAvgKWh:        math.Round(monthlyAvg*100) / 100,
		TotalSavingsBRL:      totalSavings,
		MonthlyAvgSavingsBRL: avgSavings,
		MonthlyGeneration:    months,
	}
}

func (s *Server) handleCalculateSolarEconomy(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req solarCalculationRequest
	if err := decodeJSON(c, &req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	se, err := s.solar.Calculate(c.Request.Context(), c.Param("id"), userID, req.PanelPowerW, req.PanelCount, req.TariffBRLPerKWh)
	if errors.Is(err, service.ErrEnderecoNotFound) {
		writeError(c, http.StatusNotFound, "endereco not found")
		return
	}
	if errors.Is(err, service.ErrNoSolarStationAvailable) {
		writeError(c, http.StatusServiceUnavailable, "no weather station with solar data available")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusCreated, solarEconomyToResponse(se))
}

func (s *Server) handleGetLatestSolarEconomy(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	se, err := s.solar.Latest(c.Request.Context(), c.Param("id"), userID)
	if errors.Is(err, service.ErrEnderecoNotFound) || errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "no solar economy calculated for this endereco")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, solarEconomyToResponse(se))
}
