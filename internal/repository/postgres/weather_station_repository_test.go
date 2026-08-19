package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Davi-OS/rain-saver/internal/repository"
)

func TestWeatherStationRepository_NearestPicksClosest(t *testing.T) {
	pool := testPool(t)
	repo := NewWeatherStationRepository(pool)
	ctx := context.Background()

	// São Paulo (A701) and Belo Horizonte (A652) — real INMET coordinates,
	// far enough apart that Haversine distance is unambiguous.
	_, err := pool.Exec(ctx, `INSERT INTO weather_stations (code, name, latitude, longitude, uf) VALUES
		('A701', 'SAO PAULO - MIRANTE', -23.4977, -46.6202, 'SP'),
		('A652', 'BELO HORIZONTE - CERCADINHO', -19.9392, -43.9386, 'MG')`)
	if err != nil {
		t.Fatalf("seed stations: %v", err)
	}

	// A point near São Paulo should resolve to A701.
	station, distanceKm, err := repo.Nearest(ctx, -23.55, -46.63)
	if err != nil {
		t.Fatalf("Nearest() error = %v", err)
	}
	if station.Code != "A701" {
		t.Errorf("Nearest() code = %q, want A701", station.Code)
	}
	if distanceKm <= 0 || distanceKm > 50 {
		t.Errorf("Nearest() distanceKm = %v, want a small positive number", distanceKm)
	}

	// A point near Belo Horizonte should resolve to A652.
	station, _, err = repo.Nearest(ctx, -19.92, -43.94)
	if err != nil {
		t.Fatalf("Nearest() error = %v", err)
	}
	if station.Code != "A652" {
		t.Errorf("Nearest() code = %q, want A652", station.Code)
	}
}

func TestWeatherStationRepository_Nearest_NoStations(t *testing.T) {
	pool := testPool(t)
	repo := NewWeatherStationRepository(pool)

	_, _, err := repo.Nearest(context.Background(), 0, 0)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Nearest() error = %v, want ErrNotFound", err)
	}
}

func TestWeatherStationRepository_MonthlyRainfall(t *testing.T) {
	pool := testPool(t)
	repo := NewWeatherStationRepository(pool)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO weather_stations (code, name, latitude, longitude, uf) VALUES ('A701', 'SAO PAULO - MIRANTE', -23.4977, -46.6202, 'SP')`)
	if err != nil {
		t.Fatalf("seed station: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO station_monthly_rainfall (station_code, year, month, rainfall_mm) VALUES
		('A701', 2025, 1, 100),
		('A701', 2025, 2, 90),
		('A701', 2025, 3, 80),
		('A701', 2026, 1, 70)`)
	if err != nil {
		t.Fatalf("seed monthly rainfall: %v", err)
	}

	got, err := repo.MonthlyRainfall(ctx, "A701", 3)
	if err != nil {
		t.Fatalf("MonthlyRainfall() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(got), got)
	}
	// Most recent 3 rows, chronological order: 2025-02, 2025-03, 2026-01.
	want := [][2]int{{2025, 2}, {2025, 3}, {2026, 1}}
	for i, w := range want {
		if got[i].Year != w[0] || got[i].Month != w[1] {
			t.Errorf("row[%d] = (%d, %d), want (%d, %d)", i, got[i].Year, got[i].Month, w[0], w[1])
		}
	}
}

func TestWeatherStationRepository_MonthlyRainfall_UnknownStation(t *testing.T) {
	pool := testPool(t)
	repo := NewWeatherStationRepository(pool)

	got, err := repo.MonthlyRainfall(context.Background(), "does-not-exist", 12)
	if err != nil {
		t.Fatalf("MonthlyRainfall() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d rows, want 0", len(got))
	}
}
