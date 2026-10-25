package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Davi-OS/rain-saver/internal/geocoding"
)

func TestEnderecoService_CreateSetsUserIDFromInput(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo, &fakeGeocoder{})

	e, err := svc.Create(context.Background(), CreateEnderecoInput{
		UserID:     "user-1",
		Logradouro: "Av. Paulista",
		Number:     "1000",
		Latitude:   ptr(-23.5),
		Longitude:  ptr(-46.6),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if e.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", e.UserID)
	}
}

func TestEnderecoService_GetReturnsNotFoundForWrongOwner(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo, &fakeGeocoder{})

	e, _ := svc.Create(context.Background(), CreateEnderecoInput{UserID: "user-1", Logradouro: "X", Number: "1"})

	if _, err := svc.Get(context.Background(), e.ID, "user-2"); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Get() with wrong owner error = %v, want ErrEnderecoNotFound", err)
	}

	got, err := svc.Get(context.Background(), e.ID, "user-1")
	if err != nil {
		t.Fatalf("Get() with correct owner error = %v", err)
	}
	if got.ID != e.ID {
		t.Errorf("Get() id = %q, want %q", got.ID, e.ID)
	}
}

func TestEnderecoService_GetReturnsNotFoundForMissingID(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo, &fakeGeocoder{})

	if _, err := svc.Get(context.Background(), "does-not-exist", "user-1"); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Get() error = %v, want ErrEnderecoNotFound", err)
	}
}

func TestEnderecoService_DeleteEnforcesOwnership(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo, &fakeGeocoder{})

	e, _ := svc.Create(context.Background(), CreateEnderecoInput{UserID: "user-1", Logradouro: "X", Number: "1"})

	if err := svc.Delete(context.Background(), e.ID, "user-2"); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Delete() with wrong owner error = %v, want ErrEnderecoNotFound", err)
	}

	if err := svc.Delete(context.Background(), e.ID, "user-1"); err != nil {
		t.Errorf("Delete() with correct owner error = %v", err)
	}

	if _, err := svc.Get(context.Background(), e.ID, "user-1"); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Get() after delete error = %v, want ErrEnderecoNotFound", err)
	}
}

func TestEnderecoService_ListScopedToUser(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo, &fakeGeocoder{})

	svc.Create(context.Background(), CreateEnderecoInput{UserID: "user-1", Logradouro: "A", Number: "1"})
	svc.Create(context.Background(), CreateEnderecoInput{UserID: "user-1", Logradouro: "B", Number: "2"})
	svc.Create(context.Background(), CreateEnderecoInput{UserID: "user-2", Logradouro: "C", Number: "3"})

	list, err := svc.List(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List() len = %d, want 2", len(list))
	}
}

func ptr(f float64) *float64 { return &f }

// fakeGeocoder returns point (or err) and records the addresses it was asked for.
type fakeGeocoder struct {
	point geocoding.Point
	err   error
	calls []geocoding.Address
}

func (f *fakeGeocoder) Geocode(_ context.Context, a geocoding.Address) (geocoding.Point, error) {
	f.calls = append(f.calls, a)
	return f.point, f.err
}

func TestEnderecoService_CreateUsesClientCoordinatesWithoutGeocoding(t *testing.T) {
	geo := &fakeGeocoder{}
	svc := NewEnderecoService(newFakeEnderecoRepository(), geo)

	e, err := svc.Create(context.Background(), CreateEnderecoInput{
		UserID: "user-1", Logradouro: "X", Number: "1", Latitude: ptr(-10), Longitude: ptr(-20),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if e.Latitude != -10 || e.Longitude != -20 {
		t.Errorf("coords = %v,%v, want the client's", e.Latitude, e.Longitude)
	}
	if len(geo.calls) != 0 {
		t.Errorf("geocoder called %d times, want 0", len(geo.calls))
	}
}

func TestEnderecoService_CreateGeocodesWhenCoordinatesMissing(t *testing.T) {
	geo := &fakeGeocoder{point: geocoding.Point{Latitude: -23.56, Longitude: -46.65}}
	svc := NewEnderecoService(newFakeEnderecoRepository(), geo)

	e, err := svc.Create(context.Background(), CreateEnderecoInput{
		UserID: "user-1", Logradouro: " Av. Paulista ", Number: "1000", Bairro: "Bela Vista",
		Cep: "01310-100", Cidade: "São Paulo", Estado: "sp",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if e.Latitude != -23.56 || e.Longitude != -46.65 {
		t.Errorf("coords = %v,%v, want the geocoded ones", e.Latitude, e.Longitude)
	}
	if e.Estado != "SP" || e.Logradouro != "Av. Paulista" || e.Bairro != "Bela Vista" {
		t.Errorf("fields not normalized/kept: %+v", e)
	}
	if len(geo.calls) != 1 || geo.calls[0].Cidade != "São Paulo" || geo.calls[0].Estado != "SP" {
		t.Errorf("geocoder calls = %+v", geo.calls)
	}
}

func TestEnderecoService_CreateGeocodingErrors(t *testing.T) {
	in := CreateEnderecoInput{UserID: "user-1", Logradouro: "X", Number: "1", Cidade: "C", Estado: "UF"}
	tests := []struct {
		name string
		geo  geocoding.Geocoder
		want error
	}{
		{"not found", &fakeGeocoder{err: geocoding.ErrNotFound}, ErrAddressNotFound},
		{"provider down", &fakeGeocoder{err: geocoding.ErrUnavailable}, ErrGeocodingUnavailable},
		{"no geocoder configured", nil, ErrAddressNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeEnderecoRepository()
			if _, err := NewEnderecoService(repo, tt.geo).Create(context.Background(), in); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if list, _ := repo.ListByUserID(context.Background(), "user-1"); len(list) != 0 {
				t.Error("nothing may be stored when the address cannot be located")
			}
		})
	}
}
