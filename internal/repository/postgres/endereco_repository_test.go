package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

func TestEnderecoRepository_CreateAndGetByID(t *testing.T) {
	pool := testPool(t)
	repo := NewEnderecoRepository(pool)
	ctx := context.Background()

	in := &domain.Endereco{
		UserID:     "11111111-1111-1111-1111-111111111111",
		Logradouro: "Av. Paulista",
		Number:     "1000",
		Cep:        "01310-100",
		Latitude:   -23.561684,
		Longitude:  -46.655981,
		Cidade:     "São Paulo",
		Estado:     "SP",
	}

	created, err := repo.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Logradouro != in.Logradouro || got.UserID != in.UserID || got.Latitude != in.Latitude {
		t.Errorf("GetByID() = %+v, want fields matching %+v", got, in)
	}
}

func TestEnderecoRepository_GetByID_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewEnderecoRepository(pool)

	_, err := repo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestEnderecoRepository_ListByUserID(t *testing.T) {
	pool := testPool(t)
	repo := NewEnderecoRepository(pool)
	ctx := context.Background()

	const userA = "11111111-1111-1111-1111-111111111111"
	const userB = "22222222-2222-2222-2222-222222222222"

	repo.Create(ctx, &domain.Endereco{UserID: userA, Logradouro: "A", Number: "1", Latitude: 1, Longitude: 1})
	repo.Create(ctx, &domain.Endereco{UserID: userA, Logradouro: "B", Number: "2", Latitude: 2, Longitude: 2})
	repo.Create(ctx, &domain.Endereco{UserID: userB, Logradouro: "C", Number: "3", Latitude: 3, Longitude: 3})

	list, err := repo.ListByUserID(ctx, userA)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByUserID() len = %d, want 2", len(list))
	}
}

func TestEnderecoRepository_DeleteEnforcesOwnership(t *testing.T) {
	pool := testPool(t)
	repo := NewEnderecoRepository(pool)
	ctx := context.Background()

	const owner = "11111111-1111-1111-1111-111111111111"
	const other = "22222222-2222-2222-2222-222222222222"

	e, err := repo.Create(ctx, &domain.Endereco{UserID: owner, Logradouro: "A", Number: "1", Latitude: 1, Longitude: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.Delete(ctx, e.ID, other); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete() with wrong owner error = %v, want ErrNotFound", err)
	}

	// The row must still be there — a wrong-owner delete must not have
	// silently succeeded against a WHERE clause that only matched on id.
	if _, err := repo.GetByID(ctx, e.ID); err != nil {
		t.Errorf("GetByID() after wrong-owner delete error = %v, want the row to still exist", err)
	}

	if err := repo.Delete(ctx, e.ID, owner); err != nil {
		t.Errorf("Delete() with correct owner error = %v", err)
	}

	if _, err := repo.GetByID(ctx, e.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByID() after delete error = %v, want ErrNotFound", err)
	}
}
