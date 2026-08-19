package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

func newTestEndereco() *domain.Endereco {
	return &domain.Endereco{UserID: "user-1", Logradouro: "Av. Paulista", Number: "1000", Latitude: -23.5, Longitude: -46.6}
}

func testStation() *domain.WeatherStation {
	return &domain.WeatherStation{Code: "A701", Name: "SAO PAULO - MIRANTE", Latitude: -23.4977, Longitude: -46.6202}
}

func TestPluvialService_CalculateComputesCapturedLiters(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	stations := &fakeWeatherStationRepository{
		station:    testStation(),
		distanceKm: 8.456,
		monthly: []domain.StationMonthlyRainfall{
			{StationCode: "A701", Year: 2026, Month: 1, RainfallMM: 20},
			{StationCode: "A701", Year: 2026, Month: 2, RainfallMM: 5},
		},
	}
	svc := NewPluvialService(enderecoRepo, economyRepo, stations)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	pe, err := svc.Calculate(context.Background(), e.ID, e.UserID, 100)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if pe.StationCode != "A701" {
		t.Errorf("StationCode = %q, want A701", pe.StationCode)
	}
	if pe.DistanceKm != 8.46 {
		t.Errorf("DistanceKm = %v, want 8.46 (rounded)", pe.DistanceKm)
	}
	if len(pe.MonthlyCaptation) != 2 {
		t.Fatalf("got %d months, want 2: %+v", len(pe.MonthlyCaptation), pe.MonthlyCaptation)
	}

	jan := pe.MonthlyCaptation[0]
	if jan.Month != 1 || jan.RainfallMM != 20 || jan.CapturedLiters != 2000 {
		t.Errorf("january bucket = %+v, want month=1 rainfall=20 captured=2000", jan)
	}

	feb := pe.MonthlyCaptation[1]
	if feb.Month != 2 || feb.RainfallMM != 5 || feb.CapturedLiters != 500 {
		t.Errorf("february bucket = %+v, want month=2 rainfall=5 captured=500", feb)
	}
}

func TestPluvialService_CalculateReturnsNotFoundForWrongOwner(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	stations := &fakeWeatherStationRepository{station: testStation()}
	svc := NewPluvialService(enderecoRepo, economyRepo, stations)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Calculate(context.Background(), e.ID, "someone-else", 100); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Calculate() error = %v, want ErrEnderecoNotFound", err)
	}
}

func TestPluvialService_CalculateReturnsErrNoStationAvailable(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	stations := &fakeWeatherStationRepository{nearestErr: repository.ErrNotFound}
	svc := NewPluvialService(enderecoRepo, economyRepo, stations)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Calculate(context.Background(), e.ID, e.UserID, 100); !errors.Is(err, ErrNoStationAvailable) {
		t.Errorf("Calculate() error = %v, want ErrNoStationAvailable", err)
	}
}

func TestPluvialService_LatestReturnsNotFoundWhenNoneCalculated(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	svc := NewPluvialService(enderecoRepo, economyRepo, &fakeWeatherStationRepository{})

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Latest(context.Background(), e.ID, e.UserID); err == nil {
		t.Error("Latest() error = nil, want a not-found error")
	}
}
