package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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

// seedSolarMonths inserts n consecutive months of solar data for a station
// starting at (year, month), skipping the given offsets (0 = the first month).
func seedSolarMonths(t *testing.T, pool *pgxpool.Pool, code string, year, month, n int, skip ...int) {
	t.Helper()
	skipped := map[int]bool{}
	for _, s := range skip {
		skipped[s] = true
	}
	for i := 0; i < n; i++ {
		if skipped[i] {
			continue
		}
		idx := year*12 + (month - 1) + i
		_, err := pool.Exec(context.Background(),
			`INSERT INTO station_monthly_solar (station_code, year, month, avg_daily_kwh_m2, valid_days) VALUES ($1, $2, $3, 5.5, 30)`,
			code, idx/12, idx%12+1)
		if err != nil {
			t.Fatalf("seedSolarMonths(%s): %v", code, err)
		}
	}
}

func TestWeatherStationRepository_NearestWithSolarOnlyUsesCompleteYears(t *testing.T) {
	pool := testPool(t)
	repo := NewWeatherStationRepository(pool)
	ctx := context.Background()

	// All four stations sit near São Paulo, closest first, so a station is
	// only chosen if the closer ones are rejected.
	_, err := pool.Exec(ctx, `INSERT INTO weather_stations (code, name, latitude, longitude, uf) VALUES
		('NOSOLAR', 'NO SOLAR DATA', -23.50, -46.60, 'SP'),
		('SHORT', 'ELEVEN MONTHS', -23.52, -46.62, 'SP'),
		('GAP', 'TWELVE MONTHS WITH A GAP', -23.54, -46.64, 'SP'),
		('FULL', 'TWELVE CONSECUTIVE MONTHS', -23.56, -46.66, 'SP'),
		('FAR', 'BELO HORIZONTE', -19.9392, -43.9386, 'MG')`)
	if err != nil {
		t.Fatalf("seed stations: %v", err)
	}

	// Nobody has solar data yet.
	if _, _, err := repo.NearestWithSolar(ctx, -23.55, -46.63); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("NearestWithSolar() with no solar data error = %v, want ErrNotFound", err)
	}

	seedSolarMonths(t, pool, "SHORT", 2025, 9, 11)
	// 13 months starting in 2025-06 with the 3rd one missing: 12 rows, but
	// the latest 12 months (2025-07..2026-06) include the gap at 2025-08.
	seedSolarMonths(t, pool, "GAP", 2025, 6, 13, 2)
	if _, _, err := repo.NearestWithSolar(ctx, -23.55, -46.63); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("NearestWithSolar() with only short/gappy stations error = %v, want ErrNotFound", err)
	}

	// 12 consecutive months across a year boundary: 2025-09..2026-08.
	seedSolarMonths(t, pool, "FULL", 2025, 9, 12)
	// Complete too, but much farther away.
	seedSolarMonths(t, pool, "FAR", 2025, 9, 12)

	station, _, err := repo.NearestWithSolar(ctx, -23.55, -46.63)
	if err != nil {
		t.Fatalf("NearestWithSolar() error = %v", err)
	}
	if station.Code != "FULL" {
		t.Errorf("NearestWithSolar() code = %q, want FULL (the closest station with a complete year)", station.Code)
	}
}

func TestWeatherStationRepository_MonthlySolar(t *testing.T) {
	pool := testPool(t)
	repo := NewWeatherStationRepository(pool)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO weather_stations (code, name, latitude, longitude, uf) VALUES ('A701', 'SAO PAULO - MIRANTE', -23.4977, -46.6202, 'SP')`)
	if err != nil {
		t.Fatalf("seed station: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO station_monthly_solar (station_code, year, month, avg_daily_kwh_m2, valid_days) VALUES
		('A701', 2025, 1, 5.1, 30),
		('A701', 2025, 2, 4.9, 28),
		('A701', 2026, 1, 5.3, 31)`)
	if err != nil {
		t.Fatalf("seed monthly solar: %v", err)
	}

	got, err := repo.MonthlySolar(ctx, "A701", 2)
	if err != nil {
		t.Fatalf("MonthlySolar() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(got), got)
	}
	if got[0].Year != 2025 || got[0].Month != 2 || got[0].AvgDailyKWhM2 != 4.9 {
		t.Errorf("row[0] = %+v, want 2025-02 4.9", got[0])
	}
	if got[1].Year != 2026 || got[1].Month != 1 {
		t.Errorf("row[1] = %+v, want 2026-01", got[1])
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
