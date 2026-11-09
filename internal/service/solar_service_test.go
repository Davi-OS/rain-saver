package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

func TestSolarService_CalculateComputesGeneration(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakeSolarEconomyRepository()
	stations := &fakeWeatherStationRepository{
		station:    testStation(),
		distanceKm: 8.456,
		solar: []domain.StationMonthlySolar{
			{StationCode: "A701", Year: 2025, Month: 4, AvgDailyKWhM2: 5},     // 30 days
			{StationCode: "A701", Year: 2024, Month: 2, AvgDailyKWhM2: 4.123}, // leap year: 29 days
		},
	}
	svc := NewSolarService(enderecoRepo, economyRepo, stations)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	// 10 panels of 550 W = 5.5 kWp.
	se, err := svc.Calculate(context.Background(), e.ID, e.UserID, 550, 10, nil)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if se.SystemKWp != 5.5 {
		t.Errorf("SystemKWp = %v, want 5.5", se.SystemKWp)
	}
	if se.LossFactor != 0.15 {
		t.Errorf("LossFactor = %v, want 0.15", se.LossFactor)
	}
	if se.StationCode != "A701" || se.DistanceKm != 8.46 {
		t.Errorf("StationCode/DistanceKm = %q/%v, want A701/8.46", se.StationCode, se.DistanceKm)
	}
	if len(se.MonthlyGeneration) != 2 {
		t.Fatalf("got %d months, want 2: %+v", len(se.MonthlyGeneration), se.MonthlyGeneration)
	}

	// 5.5 × 5 × 30 × 0.85 = 701.25
	if got := se.MonthlyGeneration[0].GeneratedKWh; got != 701.25 {
		t.Errorf("april GeneratedKWh = %v, want 701.25", got)
	}
	// 5.5 × 4.123 × 29 × 0.85 = 558.98 (rounded); irradiation itself is
	// reported rounded to 4.12.
	feb := se.MonthlyGeneration[1]
	if feb.GeneratedKWh != 558.98 || feb.AvgDailyKWhM2 != 4.12 {
		t.Errorf("february = %+v, want generated=558.98 irradiation=4.12", feb)
	}
	if got := se.TotalKWh(); got != 701.25+558.98 {
		t.Errorf("TotalKWh() = %v, want %v", got, 701.25+558.98)
	}
}

func TestSolarService_CalculateRecordsTheTariff(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	stations := &fakeWeatherStationRepository{
		station: testStation(),
		solar:   []domain.StationMonthlySolar{{StationCode: "A701", Year: 2025, Month: 4, AvgDailyKWhM2: 5}},
	}
	svc := NewSolarService(enderecoRepo, newFakeSolarEconomyRepository(), stations)
	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	tariff := 0.95
	se, err := svc.Calculate(context.Background(), e.ID, e.UserID, 550, 10, &tariff)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if se.TariffBRLPerKWh == nil || *se.TariffBRLPerKWh != 0.95 {
		t.Errorf("TariffBRLPerKWh = %v, want 0.95", se.TariffBRLPerKWh)
	}
	// 701.25 kWh × 0.95 = 666.19
	if got, _ := se.TotalSavingsBRL(); got != 666.19 {
		t.Errorf("TotalSavingsBRL() = %v, want 666.19", got)
	}
}

func TestSolarService_CalculateReturnsNotFoundForWrongOwner(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	svc := NewSolarService(enderecoRepo, newFakeSolarEconomyRepository(), &fakeWeatherStationRepository{station: testStation()})

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Calculate(context.Background(), e.ID, "someone-else", 550, 1, nil); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Calculate() error = %v, want ErrEnderecoNotFound", err)
	}
}

func TestSolarService_CalculateReturnsErrNoSolarStationAvailable(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	svc := NewSolarService(enderecoRepo, newFakeSolarEconomyRepository(), &fakeWeatherStationRepository{nearestErr: repository.ErrNotFound})

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Calculate(context.Background(), e.ID, e.UserID, 550, 1, nil); !errors.Is(err, ErrNoSolarStationAvailable) {
		t.Errorf("Calculate() error = %v, want ErrNoSolarStationAvailable", err)
	}
}

func TestSolarService_LatestReturnsNotFoundWhenNoneCalculated(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	svc := NewSolarService(enderecoRepo, newFakeSolarEconomyRepository(), &fakeWeatherStationRepository{})

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Latest(context.Background(), e.ID, e.UserID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Latest() error = %v, want repository.ErrNotFound", err)
	}
}
