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
	createSolarEconomyQuery = `INSERT INTO solar_economy (endereco_id, user_id, panel_power_w, panel_count, system_kwp, loss_factor, period_start, period_end, station_code, distance_km, tariff_brl_per_kwh)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id::text, created_at`

	createMonthlyGenerationQuery = `INSERT INTO solar_monthly_generation (solar_economy_id, year, month, avg_daily_kwh_m2, generated_kwh)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`

	getLatestSolarEconomyQuery = `SELECT id::text, endereco_id::text, user_id::text, panel_power_w, panel_count, system_kwp, loss_factor, period_start, period_end, created_at, station_code, distance_km, tariff_brl_per_kwh
		FROM solar_economy WHERE endereco_id = $1 ORDER BY created_at DESC LIMIT 1`

	getMonthlyGenerationsQuery = `SELECT id::text, solar_economy_id::text, year, month, avg_daily_kwh_m2, generated_kwh
		FROM solar_monthly_generation WHERE solar_economy_id = $1 ORDER BY year, month`
)

type SolarEconomyRepository struct {
	pool *pgxpool.Pool
}

func NewSolarEconomyRepository(pool *pgxpool.Pool) *SolarEconomyRepository {
	return &SolarEconomyRepository{pool: pool}
}

func (r *SolarEconomyRepository) Create(ctx context.Context, se *domain.SolarEconomy, months []domain.SolarMonthlyGeneration) (*domain.SolarEconomy, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	out := *se
	if err := tx.QueryRow(ctx, createSolarEconomyQuery,
		se.EnderecoID, se.UserID, se.PanelPowerW, se.PanelCount, se.SystemKWp, se.LossFactor,
		se.PeriodStart, se.PeriodEnd, se.StationCode, se.DistanceKm, se.TariffBRLPerKWh,
	).Scan(&out.ID, &out.CreatedAt); err != nil {
		return nil, err
	}

	out.MonthlyGeneration = make([]domain.SolarMonthlyGeneration, len(months))
	for i, m := range months {
		mg := m
		mg.SolarEconomyID = out.ID
		if err := tx.QueryRow(ctx, createMonthlyGenerationQuery,
			out.ID, mg.Year, mg.Month, mg.AvgDailyKWhM2, mg.GeneratedKWh,
		).Scan(&mg.ID); err != nil {
			return nil, err
		}
		out.MonthlyGeneration[i] = mg
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SolarEconomyRepository) GetLatestByEnderecoID(ctx context.Context, enderecoID string) (*domain.SolarEconomy, error) {
	var out domain.SolarEconomy
	err := r.pool.QueryRow(ctx, getLatestSolarEconomyQuery, enderecoID).
		Scan(&out.ID, &out.EnderecoID, &out.UserID, &out.PanelPowerW, &out.PanelCount, &out.SystemKWp, &out.LossFactor,
			&out.PeriodStart, &out.PeriodEnd, &out.CreatedAt, &out.StationCode, &out.DistanceKm, &out.TariffBRLPerKWh)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, getMonthlyGenerationsQuery, out.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mg domain.SolarMonthlyGeneration
		if err := rows.Scan(&mg.ID, &mg.SolarEconomyID, &mg.Year, &mg.Month, &mg.AvgDailyKWhM2, &mg.GeneratedKWh); err != nil {
			return nil, err
		}
		out.MonthlyGeneration = append(out.MonthlyGeneration, mg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &out, nil
}
