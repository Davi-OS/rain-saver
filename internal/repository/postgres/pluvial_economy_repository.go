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
	createPluvialEconomyQuery = `INSERT INTO pluvial_economy (endereco_id, user_id, area_m2, period_start, period_end)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, created_at`

	createMonthlyCaptationQuery = `INSERT INTO pluvial_monthly_captation (pluvial_economy_id, year, month, rainfall_mm, captured_liters)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`

	getLatestPluvialEconomyQuery = `SELECT id::text, endereco_id::text, user_id::text, area_m2, period_start, period_end, created_at
		FROM pluvial_economy WHERE endereco_id = $1 ORDER BY created_at DESC LIMIT 1`

	getMonthlyCaptationsQuery = `SELECT id::text, pluvial_economy_id::text, year, month, rainfall_mm, captured_liters
		FROM pluvial_monthly_captation WHERE pluvial_economy_id = $1 ORDER BY year, month`
)

type PluvialEconomyRepository struct {
	pool *pgxpool.Pool
}

func NewPluvialEconomyRepository(pool *pgxpool.Pool) *PluvialEconomyRepository {
	return &PluvialEconomyRepository{pool: pool}
}

func (r *PluvialEconomyRepository) Create(ctx context.Context, pe *domain.PluvialEconomy, months []domain.PluvialMonthlyCaptation) (*domain.PluvialEconomy, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	out := *pe
	if err := tx.QueryRow(ctx, createPluvialEconomyQuery,
		pe.EnderecoID, pe.UserID, pe.AreaM2, pe.PeriodStart, pe.PeriodEnd,
	).Scan(&out.ID, &out.CreatedAt); err != nil {
		return nil, err
	}

	out.MonthlyCaptation = make([]domain.PluvialMonthlyCaptation, len(months))
	for i, m := range months {
		mc := m
		mc.PluvialEconomyID = out.ID
		if err := tx.QueryRow(ctx, createMonthlyCaptationQuery,
			out.ID, mc.Year, mc.Month, mc.RainfallMM, mc.CapturedLiters,
		).Scan(&mc.ID); err != nil {
			return nil, err
		}
		out.MonthlyCaptation[i] = mc
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *PluvialEconomyRepository) GetLatestByEnderecoID(ctx context.Context, enderecoID string) (*domain.PluvialEconomy, error) {
	var out domain.PluvialEconomy
	err := r.pool.QueryRow(ctx, getLatestPluvialEconomyQuery, enderecoID).
		Scan(&out.ID, &out.EnderecoID, &out.UserID, &out.AreaM2, &out.PeriodStart, &out.PeriodEnd, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, getMonthlyCaptationsQuery, out.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mc domain.PluvialMonthlyCaptation
		if err := rows.Scan(&mc.ID, &mc.PluvialEconomyID, &mc.Year, &mc.Month, &mc.RainfallMM, &mc.CapturedLiters); err != nil {
			return nil, err
		}
		out.MonthlyCaptation = append(out.MonthlyCaptation, mc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &out, nil
}
