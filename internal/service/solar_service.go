package service

import (
	"context"
	"errors"
	"time"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

const (
	solarWindowMonths = 12
	// solarLossFactor is the share of nominal output assumed lost to the
	// inverter, wiring, heat and dirt.
	solarLossFactor = 0.15
)

type SolarService struct {
	enderecos repository.EnderecoRepository
	economies repository.SolarEconomyRepository
	stations  repository.WeatherStationRepository
}

func NewSolarService(enderecos repository.EnderecoRepository, economies repository.SolarEconomyRepository, stations repository.WeatherStationRepository) *SolarService {
	return &SolarService{enderecos: enderecos, economies: economies, stations: stations}
}

// Calculate estimates a photovoltaic system's generation at the address
// from the nearest INMET station with solar data, over its last 12 compiled
// months. Each month's average daily irradiation (kWh/m²/day) equals its
// peak sun hours, since panels are rated at 1000 W/m², so:
//
//	kWh = kWp × avg_daily_kwh_m2 × days_in_month × (1 − loss)
//
// Irradiation is measured on the horizontal plane; panel tilt and
// orientation are not modelled. Persists the result like PluvialService.
// Returns ErrEnderecoNotFound if the address doesn't exist or doesn't
// belong to userID, or ErrNoSolarStationAvailable if no station has solar
// data.
func (s *SolarService) Calculate(ctx context.Context, enderecoID, userID string, panelPowerW float64, panelCount int) (*domain.SolarEconomy, error) {
	e, err := ownedEndereco(ctx, s.enderecos, enderecoID, userID)
	if err != nil {
		return nil, err
	}

	station, distanceKm, err := s.stations.NearestWithSolar(ctx, e.Latitude, e.Longitude)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNoSolarStationAvailable
	}
	if err != nil {
		return nil, err
	}

	monthly, err := s.stations.MonthlySolar(ctx, station.Code, solarWindowMonths)
	if err != nil {
		return nil, err
	}

	systemKWp := panelPowerW * float64(panelCount) / 1000
	now := time.Now().UTC()
	se := &domain.SolarEconomy{
		EnderecoID:  e.ID,
		UserID:      userID,
		PanelPowerW: panelPowerW,
		PanelCount:  panelCount,
		SystemKWp:   systemKWp,
		LossFactor:  solarLossFactor,
		PeriodStart: now.AddDate(-1, 0, 0),
		PeriodEnd:   now,
		StationCode: station.Code,
		DistanceKm:  roundToTwoDecimals(distanceKm),
	}

	months := make([]domain.SolarMonthlyGeneration, len(monthly))
	for i, m := range monthly {
		days := float64(daysInMonth(m.Year, m.Month))
		months[i] = domain.SolarMonthlyGeneration{
			Year:          m.Year,
			Month:         m.Month,
			AvgDailyKWhM2: roundToTwoDecimals(m.AvgDailyKWhM2),
			GeneratedKWh:  roundToTwoDecimals(systemKWp * m.AvgDailyKWhM2 * days * (1 - solarLossFactor)),
		}
	}

	return s.economies.Create(ctx, se, months)
}

// Latest returns the most recent calculation for the address. Returns
// ErrEnderecoNotFound if the address doesn't exist or doesn't belong to
// userID, or repository.ErrNotFound if none has ever been run.
func (s *SolarService) Latest(ctx context.Context, enderecoID, userID string) (*domain.SolarEconomy, error) {
	if _, err := ownedEndereco(ctx, s.enderecos, enderecoID, userID); err != nil {
		return nil, err
	}
	return s.economies.GetLatestByEnderecoID(ctx, enderecoID)
}

func daysInMonth(year, month int) int {
	// Day 0 of the next month is the last day of this one.
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
