package service

import (
	"context"
	"errors"
	"testing"
)

func TestEnderecoService_CreateSetsUserIDFromInput(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo)

	e, err := svc.Create(context.Background(), CreateEnderecoInput{
		UserID:     "user-1",
		Logradouro: "Av. Paulista",
		Number:     "1000",
		Latitude:   -23.5,
		Longitude:  -46.6,
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
	svc := NewEnderecoService(repo)

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
	svc := NewEnderecoService(repo)

	if _, err := svc.Get(context.Background(), "does-not-exist", "user-1"); !errors.Is(err, ErrEnderecoNotFound) {
		t.Errorf("Get() error = %v, want ErrEnderecoNotFound", err)
	}
}

func TestEnderecoService_DeleteEnforcesOwnership(t *testing.T) {
	repo := newFakeEnderecoRepository()
	svc := NewEnderecoService(repo)

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
	svc := NewEnderecoService(repo)

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
