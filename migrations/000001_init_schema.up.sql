CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- user_id references a user owned by the separate go-auth-service (its own
-- database/service boundary), so there is no local users table and no FK.
CREATE TABLE enderecos (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL,
    logradouro text NOT NULL,
    number     text NOT NULL,
    cep        text,
    latitude   double precision NOT NULL,
    longitude  double precision NOT NULL,
    cidade     text,
    estado     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_enderecos_user_id ON enderecos(user_id);

-- Append-only log: every calculation run inserts a new row rather than
-- upserting, so a calculation-history feature is free later.
CREATE TABLE pluvial_economy (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endereco_id  uuid NOT NULL REFERENCES enderecos(id) ON DELETE CASCADE,
    user_id      uuid NOT NULL,
    area_m2      double precision NOT NULL,
    period_start date NOT NULL,
    period_end   date NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_pluvial_economy_endereco_id ON pluvial_economy(endereco_id);
CREATE INDEX idx_pluvial_economy_endereco_created_at ON pluvial_economy(endereco_id, created_at DESC);

CREATE TABLE pluvial_monthly_captation (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pluvial_economy_id uuid NOT NULL REFERENCES pluvial_economy(id) ON DELETE CASCADE,
    year               int NOT NULL,
    month              int NOT NULL CHECK (month BETWEEN 1 AND 12),
    rainfall_mm        double precision NOT NULL,
    captured_liters    double precision NOT NULL,
    UNIQUE (pluvial_economy_id, year, month)
);

CREATE INDEX idx_pluvial_monthly_captation_economy_id ON pluvial_monthly_captation(pluvial_economy_id);
