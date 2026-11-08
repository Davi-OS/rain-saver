package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

func TestSolarEconomyRepository_CreatePersistsParentAndMonths(t *testing.T) {
	pool := testPool(t)
	enderecoRepo := NewEnderecoRepository(pool)
	repo := NewSolarEconomyRepository(pool)
	ctx := context.Background()

	e := createTestEndereco(t, enderecoRepo)
	createTestStation(t, pool, "A701")

	now := time.Now().UTC()
	se := &domain.SolarEconomy{
		EnderecoID:  e.ID,
		UserID:      e.UserID,
		PanelPowerW: 550,
		PanelCount:  10,
		SystemKWp:   5.5,
		LossFactor:  0.15,
		PeriodStart: now.AddDate(-1, 0, 0),
		PeriodEnd:   now,
		StationCode: "A701",
		DistanceKm:  12.3,
	}
	months := []domain.SolarMonthlyGeneration{
		{Year: 2026, Month: 1, AvgDailyKWhM2: 5.2, GeneratedKWh: 753.6},
		{Year: 2026, Month: 2, AvgDailyKWhM2: 5.0, GeneratedKWh: 654.5},
	}

	created, err := repo.Create(ctx, se, months)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" || len(created.MonthlyGeneration) != 2 {
		t.Fatalf("Create() = %+v, want an ID and 2 months", created)
	}

	got, err := repo.GetLatestByEnderecoID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetLatestByEnderecoID() error = %v", err)
	}
	if got.PanelPowerW != 550 || got.PanelCount != 10 || got.SystemKWp != 5.5 || got.LossFactor != 0.15 {
		t.Errorf("system = %v W × %d = %v kWp, loss %v; want 550 × 10 = 5.5, loss 0.15", got.PanelPowerW, got.PanelCount, got.SystemKWp, got.LossFactor)
	}
	if got.StationCode != "A701" || got.DistanceKm != 12.3 {
		t.Errorf("StationCode/DistanceKm = %q/%v, want A701/12.3", got.StationCode, got.DistanceKm)
	}
	if len(got.MonthlyGeneration) != 2 || got.MonthlyGeneration[0].Month != 1 || got.MonthlyGeneration[1].GeneratedKWh != 654.5 {
		t.Errorf("months = %+v", got.MonthlyGeneration)
	}
}

func TestSolarEconomyRepository_CreateRollsBackOnDuplicateMonth(t *testing.T) {
	pool := testPool(t)
	enderecoRepo := NewEnderecoRepository(pool)
	repo := NewSolarEconomyRepository(pool)
	ctx := context.Background()

	e := createTestEndereco(t, enderecoRepo)
	createTestStation(t, pool, "A701")

	now := time.Now().UTC()
	se := &domain.SolarEconomy{
		EnderecoID: e.ID, UserID: e.UserID, PanelPowerW: 550, PanelCount: 1, SystemKWp: 0.55, LossFactor: 0.15,
		PeriodStart: now, PeriodEnd: now, StationCode: "A701",
	}
	months := []domain.SolarMonthlyGeneration{
		{Year: 2026, Month: 1, AvgDailyKWhM2: 5, GeneratedKWh: 70},
		{Year: 2026, Month: 1, AvgDailyKWhM2: 5, GeneratedKWh: 70},
	}

	if _, err := repo.Create(ctx, se, months); err == nil {
		t.Fatal("Create() with duplicate month error = nil, want a unique-constraint error")
	}
	if _, err := repo.GetLatestByEnderecoID(ctx, e.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetLatestByEnderecoID() after rollback error = %v, want ErrNotFound", err)
	}
}
