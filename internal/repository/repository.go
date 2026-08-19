package repository

import (
	"context"
	"errors"

	"github.com/Davi-OS/rain-saver/internal/domain"
)

var ErrNotFound = errors.New("repository: not found")

type EnderecoRepository interface {
	Create(ctx context.Context, e *domain.Endereco) (*domain.Endereco, error)
	GetByID(ctx context.Context, id string) (*domain.Endereco, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Endereco, error)
	// Delete removes the address only if it belongs to userID, returning
	// ErrNotFound otherwise — ownership is enforced in the SQL WHERE clause
	// rather than as a separate lookup, to avoid a TOCTOU gap.
	Delete(ctx context.Context, id, userID string) error
}

type PluvialEconomyRepository interface {
	// Create persists the parent row and all month rows in a single
	// transaction and returns the fully populated result.
	Create(ctx context.Context, pe *domain.PluvialEconomy, months []domain.PluvialMonthlyCaptation) (*domain.PluvialEconomy, error)
	// GetLatestByEnderecoID returns the most recently created calculation
	// for the address, with MonthlyCaptation populated, or ErrNotFound if
	// none has ever been run.
	GetLatestByEnderecoID(ctx context.Context, enderecoID string) (*domain.PluvialEconomy, error)
}

type WeatherStationRepository interface {
	// Nearest returns the closest station to the given coordinates (via a
	// Haversine distance computed in SQL) and the distance in km.
	// ErrNotFound if no station is registered at all.
	Nearest(ctx context.Context, latitude, longitude float64) (*domain.WeatherStation, float64, error)
	// MonthlyRainfall returns up to the most recent `months` (year, month)
	// rows already compiled for the station, in chronological order. It
	// does not assume the table holds exactly `months` rows — filtering
	// happens at read time, so rain-saver stays decoupled from however the
	// ingestion side decides to retain history.
	MonthlyRainfall(ctx context.Context, stationCode string, months int) ([]domain.StationMonthlyRainfall, error)
}
