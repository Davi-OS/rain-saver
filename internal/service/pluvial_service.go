package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
	"github.com/Davi-OS/rain-saver/internal/weather"
)

// weatherFetcher is the slice of weather.Client this service depends on,
// so tests can substitute a fake without a network call.
type weatherFetcher interface {
	FetchDailyRainfall(ctx context.Context, latitude, longitude float64) ([]weather.DailyRainfall, error)
}

type PluvialService struct {
	enderecos repository.EnderecoRepository
	economies repository.PluvialEconomyRepository
	weather   weatherFetcher
}

func NewPluvialService(enderecos repository.EnderecoRepository, economies repository.PluvialEconomyRepository, weather weatherFetcher) *PluvialService {
	return &PluvialService{enderecos: enderecos, economies: economies, weather: weather}
}

// Calculate fetches a year of daily rainfall for the address, buckets it by
// calendar month, computes captured liters (area_m2 * rainfall_mm per
// month, 1mm over 1m² == 1L, same physics as the old backend) and persists
// the result. Returns ErrEnderecoNotFound if the address doesn't exist or
// doesn't belong to userID.
func (s *PluvialService) Calculate(ctx context.Context, enderecoID, userID string, areaM2 float64) (*domain.PluvialEconomy, error) {
	e, err := s.ownedEndereco(ctx, enderecoID, userID)
	if err != nil {
		return nil, err
	}

	daily, err := s.weather.FetchDailyRainfall(ctx, e.Latitude, e.Longitude)
	if err != nil {
		return nil, errors.Join(ErrWeatherProvider, err)
	}

	monthly := weather.BucketByCalendarMonth(daily)

	now := time.Now().UTC()
	pe := &domain.PluvialEconomy{
		EnderecoID:  e.ID,
		UserID:      userID,
		AreaM2:      areaM2,
		PeriodStart: now.AddDate(-1, 0, 0),
		PeriodEnd:   now,
	}

	months := make([]domain.PluvialMonthlyCaptation, len(monthly))
	for i, m := range monthly {
		months[i] = domain.PluvialMonthlyCaptation{
			Year:           m.Year,
			Month:          m.Month,
			RainfallMM:     roundToTwoDecimals(m.RainfallMM),
			CapturedLiters: roundToTwoDecimals(areaM2 * m.RainfallMM),
		}
	}

	return s.economies.Create(ctx, pe, months)
}

// Latest returns the most recent calculation for the address. Returns
// ErrEnderecoNotFound if the address doesn't exist or doesn't belong to
// userID, or ErrNotFound-wrapping behavior from the repository if no
// calculation has ever been run for it.
func (s *PluvialService) Latest(ctx context.Context, enderecoID, userID string) (*domain.PluvialEconomy, error) {
	if _, err := s.ownedEndereco(ctx, enderecoID, userID); err != nil {
		return nil, err
	}
	return s.economies.GetLatestByEnderecoID(ctx, enderecoID)
}

func (s *PluvialService) ownedEndereco(ctx context.Context, enderecoID, userID string) (*domain.Endereco, error) {
	e, err := s.enderecos.GetByID(ctx, enderecoID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrEnderecoNotFound
	}
	if err != nil {
		return nil, err
	}
	if e.UserID != userID {
		return nil, ErrEnderecoNotFound
	}
	return e, nil
}

func roundToTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}
