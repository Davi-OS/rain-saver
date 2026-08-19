package service

import (
	"context"
	"strconv"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

type fakeEnderecoRepository struct {
	byID    map[string]domain.Endereco
	nextID  int
	deleted map[string]bool
}

func newFakeEnderecoRepository() *fakeEnderecoRepository {
	return &fakeEnderecoRepository{byID: map[string]domain.Endereco{}, deleted: map[string]bool{}}
}

func (f *fakeEnderecoRepository) Create(ctx context.Context, e *domain.Endereco) (*domain.Endereco, error) {
	f.nextID++
	out := *e
	out.ID = idFromInt(f.nextID)
	f.byID[out.ID] = out
	return &out, nil
}

func (f *fakeEnderecoRepository) GetByID(ctx context.Context, id string) (*domain.Endereco, error) {
	e, ok := f.byID[id]
	if !ok || f.deleted[id] {
		return nil, repository.ErrNotFound
	}
	return &e, nil
}

func (f *fakeEnderecoRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Endereco, error) {
	var out []domain.Endereco
	for _, e := range f.byID {
		if e.UserID == userID && !f.deleted[e.ID] {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeEnderecoRepository) Delete(ctx context.Context, id, userID string) error {
	e, ok := f.byID[id]
	if !ok || f.deleted[id] || e.UserID != userID {
		return repository.ErrNotFound
	}
	f.deleted[id] = true
	return nil
}

func idFromInt(n int) string {
	return "id-" + strconv.Itoa(n)
}

type fakePluvialEconomyRepository struct {
	latestByEndereco map[string]domain.PluvialEconomy
	nextID           int
}

func newFakePluvialEconomyRepository() *fakePluvialEconomyRepository {
	return &fakePluvialEconomyRepository{latestByEndereco: map[string]domain.PluvialEconomy{}}
}

func (f *fakePluvialEconomyRepository) Create(ctx context.Context, pe *domain.PluvialEconomy, months []domain.PluvialMonthlyCaptation) (*domain.PluvialEconomy, error) {
	f.nextID++
	out := *pe
	out.ID = idFromInt(f.nextID)
	out.MonthlyCaptation = months
	f.latestByEndereco[pe.EnderecoID] = out
	return &out, nil
}

func (f *fakePluvialEconomyRepository) GetLatestByEnderecoID(ctx context.Context, enderecoID string) (*domain.PluvialEconomy, error) {
	pe, ok := f.latestByEndereco[enderecoID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &pe, nil
}

type fakeWeatherStationRepository struct {
	station    *domain.WeatherStation
	distanceKm float64
	nearestErr error
	monthly    []domain.StationMonthlyRainfall
	monthlyErr error
}

func (f *fakeWeatherStationRepository) Nearest(ctx context.Context, latitude, longitude float64) (*domain.WeatherStation, float64, error) {
	if f.nearestErr != nil {
		return nil, 0, f.nearestErr
	}
	return f.station, f.distanceKm, nil
}

func (f *fakeWeatherStationRepository) MonthlyRainfall(ctx context.Context, stationCode string, months int) ([]domain.StationMonthlyRainfall, error) {
	return f.monthly, f.monthlyErr
}
