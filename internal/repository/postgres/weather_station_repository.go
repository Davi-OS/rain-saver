package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

const (
	// The Haversine formula in plain SQL — the INMET network has on the
	// order of ~1000 stations, so a full scan + ORDER BY LIMIT 1 is
	// trivial cost and doesn't justify a spatial extension like PostGIS.
	nearestStationQuery = `SELECT code, name, latitude, longitude, uf, created_at,
		(6371 * acos(cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2)) + sin(radians($1)) * sin(radians(latitude)))) AS distance_km
		FROM weather_stations
		ORDER BY distance_km ASC
		LIMIT 1`

	monthlyRainfallQuery = `SELECT station_code, year, month, rainfall_mm FROM (
		SELECT station_code, year, month, rainfall_mm
		FROM station_monthly_rainfall
		WHERE station_code = $1
		ORDER BY year DESC, month DESC
		LIMIT $2
	) recent
	ORDER BY year, month`
)

type WeatherStationRepository struct {
	pool *pgxpool.Pool
}

func NewWeatherStationRepository(pool *pgxpool.Pool) *WeatherStationRepository {
	return &WeatherStationRepository{pool: pool}
}

func (r *WeatherStationRepository) Nearest(ctx context.Context, latitude, longitude float64) (*domain.WeatherStation, float64, error) {
	var s domain.WeatherStation
	var distanceKm float64
	err := r.pool.QueryRow(ctx, nearestStationQuery, latitude, longitude).
		Scan(&s.Code, &s.Name, &s.Latitude, &s.Longitude, &s.UF, &s.CreatedAt, &distanceKm)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, repository.ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	return &s, distanceKm, nil
}

func (r *WeatherStationRepository) MonthlyRainfall(ctx context.Context, stationCode string, months int) ([]domain.StationMonthlyRainfall, error) {
	rows, err := r.pool.Query(ctx, monthlyRainfallQuery, stationCode, months)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.StationMonthlyRainfall
	for rows.Next() {
		var m domain.StationMonthlyRainfall
		if err := rows.Scan(&m.StationCode, &m.Year, &m.Month, &m.RainfallMM); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
