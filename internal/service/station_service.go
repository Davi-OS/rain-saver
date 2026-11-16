package service

import (
	"context"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

type StationService struct {
	stations repository.WeatherStationRepository
}

func NewStationService(stations repository.WeatherStationRepository) *StationService {
	return &StationService{stations: stations}
}

// List returns every station the ingestor has registered.
func (s *StationService) List(ctx context.Context) ([]domain.WeatherStation, error) {
	return s.stations.List(ctx)
}
