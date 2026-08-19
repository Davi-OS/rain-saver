package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

func createTestEndereco(t *testing.T, repo *EnderecoRepository) *domain.Endereco {
	t.Helper()
	e, err := repo.Create(context.Background(), &domain.Endereco{
		UserID:     "11111111-1111-1111-1111-111111111111",
		Logradouro: "Av. Paulista",
		Number:     "1000",
		Latitude:   -23.5,
		Longitude:  -46.6,
	})
	if err != nil {
		t.Fatalf("createTestEndereco: %v", err)
	}
	return e
}

// createTestStation inserts a station row so pluvial_economy.station_code's
// FK constraint is satisfiable in tests.
func createTestStation(t *testing.T, pool *pgxpool.Pool, code string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO weather_stations (code, name, latitude, longitude, uf) VALUES ($1, 'TEST STATION', -23.5, -46.6, 'SP')`, code)
	if err != nil {
		t.Fatalf("createTestStation: %v", err)
	}
}

func TestPluvialEconomyRepository_CreatePersistsParentAndMonths(t *testing.T) {
	pool := testPool(t)
	enderecoRepo := NewEnderecoRepository(pool)
	repo := NewPluvialEconomyRepository(pool)
	ctx := context.Background()

	e := createTestEndereco(t, enderecoRepo)
	createTestStation(t, pool, "A701")

	now := time.Now().UTC()
	pe := &domain.PluvialEconomy{
		EnderecoID:  e.ID,
		UserID:      e.UserID,
		AreaM2:      100,
		PeriodStart: now.AddDate(-1, 0, 0),
		PeriodEnd:   now,
		StationCode: "A701",
		DistanceKm:  12.3,
	}
	months := []domain.PluvialMonthlyCaptation{
		{Year: 2026, Month: 1, RainfallMM: 10, CapturedLiters: 1000},
		{Year: 2026, Month: 2, RainfallMM: 20, CapturedLiters: 2000},
	}

	created, err := repo.Create(ctx, pe, months)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if len(created.MonthlyCaptation) != 2 {
		t.Fatalf("Create() returned %d months, want 2", len(created.MonthlyCaptation))
	}

	got, err := repo.GetLatestByEnderecoID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetLatestByEnderecoID() error = %v", err)
	}
	if len(got.MonthlyCaptation) != 2 {
		t.Fatalf("GetLatestByEnderecoID() months = %d, want 2", len(got.MonthlyCaptation))
	}
	if got.MonthlyCaptation[0].Month != 1 || got.MonthlyCaptation[1].Month != 2 {
		t.Errorf("months not in order: %+v", got.MonthlyCaptation)
	}
	if got.StationCode != "A701" || got.DistanceKm != 12.3 {
		t.Errorf("StationCode/DistanceKm = %q/%v, want A701/12.3", got.StationCode, got.DistanceKm)
	}
}

func TestPluvialEconomyRepository_CreateRollsBackOnDuplicateMonth(t *testing.T) {
	pool := testPool(t)
	enderecoRepo := NewEnderecoRepository(pool)
	repo := NewPluvialEconomyRepository(pool)
	ctx := context.Background()

	e := createTestEndereco(t, enderecoRepo)
	createTestStation(t, pool, "A701")

	now := time.Now().UTC()
	pe := &domain.PluvialEconomy{
		EnderecoID:  e.ID,
		UserID:      e.UserID,
		AreaM2:      100,
		PeriodStart: now.AddDate(-1, 0, 0),
		PeriodEnd:   now,
		StationCode: "A701",
		DistanceKm:  12.3,
	}
	// Duplicate (year, month) pair violates the unique constraint mid-batch
	// — the whole parent+children insert must roll back, leaving nothing.
	months := []domain.PluvialMonthlyCaptation{
		{Year: 2026, Month: 1, RainfallMM: 10, CapturedLiters: 1000},
		{Year: 2026, Month: 1, RainfallMM: 10, CapturedLiters: 1000},
	}

	if _, err := repo.Create(ctx, pe, months); err == nil {
		t.Fatal("Create() with duplicate month error = nil, want a unique-constraint error")
	}

	if _, err := repo.GetLatestByEnderecoID(ctx, e.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetLatestByEnderecoID() after rollback error = %v, want ErrNotFound (nothing should have been persisted)", err)
	}
}

func TestPluvialEconomyRepository_GetLatestReturnsMostRecent(t *testing.T) {
	pool := testPool(t)
	enderecoRepo := NewEnderecoRepository(pool)
	repo := NewPluvialEconomyRepository(pool)
	ctx := context.Background()

	e := createTestEndereco(t, enderecoRepo)
	createTestStation(t, pool, "A701")
	now := time.Now().UTC()

	first, err := repo.Create(ctx, &domain.PluvialEconomy{
		EnderecoID: e.ID, UserID: e.UserID, AreaM2: 50, PeriodStart: now, PeriodEnd: now, StationCode: "A701",
	}, []domain.PluvialMonthlyCaptation{{Year: 2025, Month: 1, RainfallMM: 1, CapturedLiters: 50}})
	if err != nil {
		t.Fatalf("Create() first error = %v", err)
	}

	second, err := repo.Create(ctx, &domain.PluvialEconomy{
		EnderecoID: e.ID, UserID: e.UserID, AreaM2: 75, PeriodStart: now, PeriodEnd: now, StationCode: "A701",
	}, []domain.PluvialMonthlyCaptation{{Year: 2026, Month: 1, RainfallMM: 2, CapturedLiters: 150}})
	if err != nil {
		t.Fatalf("Create() second error = %v", err)
	}

	got, err := repo.GetLatestByEnderecoID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetLatestByEnderecoID() error = %v", err)
	}
	if got.ID != second.ID {
		t.Errorf("GetLatestByEnderecoID() returned %q (area=%v), want the most recent %q (area=%v); first was %q",
			got.ID, got.AreaM2, second.ID, second.AreaM2, first.ID)
	}
}

func TestPluvialEconomyRepository_GetLatest_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewPluvialEconomyRepository(pool)

	_, err := repo.GetLatestByEnderecoID(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetLatestByEnderecoID() error = %v, want ErrNotFound", err)
	}
}
