package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/geocoding"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

type EnderecoService struct {
	enderecos repository.EnderecoRepository
	geocoder  geocoding.Geocoder
}

// NewEnderecoService wires the service. geocoder locates addresses created
// without coordinates; nil means coordinates are always required.
func NewEnderecoService(enderecos repository.EnderecoRepository, geocoder geocoding.Geocoder) *EnderecoService {
	return &EnderecoService{enderecos: enderecos, geocoder: geocoder}
}

type CreateEnderecoInput struct {
	UserID     string
	Logradouro string
	Number     string
	Bairro     string
	Cep        string
	// Latitude/Longitude are optional: when either is nil the address is
	// geocoded from its other fields.
	Latitude  *float64
	Longitude *float64
	Cidade    string
	Estado    string
}

func (s *EnderecoService) Create(ctx context.Context, in CreateEnderecoInput) (*domain.Endereco, error) {
	e := &domain.Endereco{
		UserID:     in.UserID,
		Logradouro: strings.TrimSpace(in.Logradouro),
		Number:     strings.TrimSpace(in.Number),
		Bairro:     strings.TrimSpace(in.Bairro),
		Cep:        strings.TrimSpace(in.Cep),
		Cidade:     strings.TrimSpace(in.Cidade),
		Estado:     strings.ToUpper(strings.TrimSpace(in.Estado)),
	}

	if in.Latitude != nil && in.Longitude != nil {
		e.Latitude, e.Longitude = *in.Latitude, *in.Longitude
	} else {
		p, err := s.locate(ctx, e)
		if err != nil {
			return nil, err
		}
		e.Latitude, e.Longitude = p.Latitude, p.Longitude
	}
	return s.enderecos.Create(ctx, e)
}

func (s *EnderecoService) locate(ctx context.Context, e *domain.Endereco) (geocoding.Point, error) {
	if s.geocoder == nil {
		return geocoding.Point{}, ErrAddressNotFound
	}
	p, err := s.geocoder.Geocode(ctx, geocoding.Address{
		Logradouro: e.Logradouro, Number: e.Number, Bairro: e.Bairro,
		Cidade: e.Cidade, Estado: e.Estado, Cep: e.Cep,
	})
	switch {
	case errors.Is(err, geocoding.ErrNotFound):
		return geocoding.Point{}, ErrAddressNotFound
	case err != nil:
		return geocoding.Point{}, errors.Join(ErrGeocodingUnavailable, err)
	}
	return p, nil
}

func (s *EnderecoService) List(ctx context.Context, userID string) ([]domain.Endereco, error) {
	return s.enderecos.ListByUserID(ctx, userID)
}

// Get returns the address only if it belongs to userID; otherwise
// ErrEnderecoNotFound, indistinguishable from a nonexistent id.
func (s *EnderecoService) Get(ctx context.Context, id, userID string) (*domain.Endereco, error) {
	e, err := s.enderecos.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrEnderecoNotFound
	}
	if err != nil {
		return nil, err
	}
	if e.UserID != userID {
		return nil, ErrEnderecoNotFound
	}
	return e, nil
}

func (s *EnderecoService) Delete(ctx context.Context, id, userID string) error {
	err := s.enderecos.Delete(ctx, id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrEnderecoNotFound
	}
	return err
}
