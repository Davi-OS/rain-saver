package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Davi-OS/rain-saver/internal/domain"
	rainhttp "github.com/Davi-OS/rain-saver/internal/http"
	"github.com/Davi-OS/rain-saver/internal/repository"
	"github.com/Davi-OS/rain-saver/internal/service"
)

type stubStationRepo struct {
	repository.WeatherStationRepository
	stations []domain.WeatherStation
	err      error
}

func (s stubStationRepo) List(context.Context) ([]domain.WeatherStation, error) {
	return s.stations, s.err
}

func newStationServer(repo stubStationRepo) http.Handler {
	return rainhttp.NewServer(nil, nil, nil, service.NewStationService(repo), nil, nil)
}

func TestListStations_PublicAndShaped(t *testing.T) {
	srv := newStationServer(stubStationRepo{stations: []domain.WeatherStation{
		{Code: "A701", Name: "SAO PAULO - MIRANTE", Latitude: -23.4977, Longitude: -46.6202, UF: "SP"},
	}})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/estacoes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without a token", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["code"] != "A701" || got[0]["uf"] != "SP" || got[0]["latitude"] != -23.4977 {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestListStations_EmptyIsArray(t *testing.T) {
	srv := newStationServer(stubStationRepo{})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/estacoes", nil))
	if rec.Body.String() != "[]" {
		t.Errorf("body = %s, want []", rec.Body.String())
	}
}

func TestListStations_RepoError(t *testing.T) {
	srv := newStationServer(stubStationRepo{err: errors.New("boom")})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/estacoes", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
