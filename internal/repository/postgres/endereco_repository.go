package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Davi-OS/rain-saver/internal/domain"
	"github.com/Davi-OS/rain-saver/internal/repository"
)

const (
	createEnderecoQuery = `INSERT INTO enderecos (user_id, logradouro, number, bairro, cep, latitude, longitude, cidade, estado)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text, user_id::text, logradouro, number, bairro, cep, latitude, longitude, cidade, estado, created_at, updated_at`

	getEnderecoByIDQuery = `SELECT id::text, user_id::text, logradouro, number, bairro, cep, latitude, longitude, cidade, estado, created_at, updated_at
		FROM enderecos WHERE id = $1`

	listEnderecosByUserIDQuery = `SELECT id::text, user_id::text, logradouro, number, bairro, cep, latitude, longitude, cidade, estado, created_at, updated_at
		FROM enderecos WHERE user_id = $1 ORDER BY created_at DESC`

	deleteEnderecoQuery = `DELETE FROM enderecos WHERE id = $1 AND user_id = $2`
)

type EnderecoRepository struct {
	pool *pgxpool.Pool
}

func NewEnderecoRepository(pool *pgxpool.Pool) *EnderecoRepository {
	return &EnderecoRepository{pool: pool}
}

func (r *EnderecoRepository) Create(ctx context.Context, e *domain.Endereco) (*domain.Endereco, error) {
	var out domain.Endereco
	err := r.pool.QueryRow(ctx, createEnderecoQuery,
		e.UserID, e.Logradouro, e.Number, e.Bairro, e.Cep, e.Latitude, e.Longitude, e.Cidade, e.Estado,
	).Scan(&out.ID, &out.UserID, &out.Logradouro, &out.Number, &out.Bairro, &out.Cep, &out.Latitude, &out.Longitude,
		&out.Cidade, &out.Estado, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *EnderecoRepository) GetByID(ctx context.Context, id string) (*domain.Endereco, error) {
	var out domain.Endereco
	err := r.pool.QueryRow(ctx, getEnderecoByIDQuery, id).
		Scan(&out.ID, &out.UserID, &out.Logradouro, &out.Number, &out.Bairro, &out.Cep, &out.Latitude, &out.Longitude,
			&out.Cidade, &out.Estado, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *EnderecoRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Endereco, error) {
	rows, err := r.pool.Query(ctx, listEnderecosByUserIDQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Endereco
	for rows.Next() {
		var e domain.Endereco
		if err := rows.Scan(&e.ID, &e.UserID, &e.Logradouro, &e.Number, &e.Bairro, &e.Cep, &e.Latitude, &e.Longitude,
			&e.Cidade, &e.Estado, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EnderecoRepository) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, deleteEnderecoQuery, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
