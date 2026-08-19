package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/weather"
)

func mustDay(t *testing.T, dateStr string, rain float64) weather.DailyRainfall {
	t.Helper()
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		t.Fatal(err)
	}
	return weather.DailyRainfall{Date: d, RainSumMM: rain}
}

func newTestEndereco() *domain.Endereco {
	return &domain.Endereco{UserID: "user-1", Logradouro: "Av. Paulista", Number: "1000", Latitude: -23.5, Longitude: -46.6}
}

func TestPluvialService_CalculateComputesCapturedLiters(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	fetcher := &fakeWeatherFetcher{daily: []weather.DailyRainfall{
		mustDay(t, "2026-01-10", 10),
		mustDay(t, "2026-01-20", 10),
		mustDay(t, "2026-02-05", 5),
	}}
	svc := NewPluvialService(enderecoRepo, economyRepo, fetcher)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	pe, err := svc.Calculate(context.Background(), e.ID, e.UserID, 100)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
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
	fetcher := &fakeWeatherFetcher{}
	svc := NewPluvialService(enderecoRepo, economyRepo, fetcher)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Calculate(context.Background(), e.ID, "someone-else", 100); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Calculate() error = %v, want ErrEnderecoNotFound", err)
	}
}

func TestPluvialService_CalculateWrapsWeatherProviderError(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	fetcher := &fakeWeatherFetcher{err: errors.New("boom")}
	svc := NewPluvialService(enderecoRepo, economyRepo, fetcher)

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Calculate(context.Background(), e.ID, e.UserID, 100); !errors.Is(err, ErrWeatherProvider) {
		t.Errorf("Calculate() error = %v, want ErrWeatherProvider", err)
	}
}

func TestPluvialService_LatestReturnsNotFoundWhenNoneCalculated(t *testing.T) {
	enderecoRepo := newFakeEnderecoRepository()
	economyRepo := newFakePluvialEconomyRepository()
	svc := NewPluvialService(enderecoRepo, economyRepo, &fakeWeatherFetcher{})

	e, _ := enderecoRepo.Create(context.Background(), newTestEndereco())

	if _, err := svc.Latest(context.Background(), e.ID, e.UserID); err == nil {
		t.Error("Latest() error = nil, want a not-found error")
	}
}
