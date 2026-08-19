package service

import (
	"context"
	"errors"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

type EnderecoService struct {
	enderecos repository.EnderecoRepository
}

func NewEnderecoService(enderecos repository.EnderecoRepository) *EnderecoService {
	return &EnderecoService{enderecos: enderecos}
}

type CreateEnderecoInput struct {
	UserID     string
	Logradouro string
	Number     string
	Cep        string
	Latitude   float64
	Longitude  float64
	Cidade     string
	Estado     string
}

func (s *EnderecoService) Create(ctx context.Context, in CreateEnderecoInput) (*domain.Endereco, error) {
	e := &domain.Endereco{
		UserID:     in.UserID,
		Logradouro: in.Logradouro,
		Number:     in.Number,
		Cep:        in.Cep,
		Latitude:   in.Latitude,
		Longitude:  in.Longitude,
		Cidade:     in.Cidade,
		Estado:     in.Estado,
	}
	return s.enderecos.Create(ctx, e)
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
