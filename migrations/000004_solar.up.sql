-- Populated by the INMET ingestion process alongside station_monthly_rainfall:
-- a month's average daily global horizontal irradiation (kWh/m²/day, i.e.
-- peak sun hours), over the days with enough hourly readings.
CREATE TABLE station_monthly_solar (
    station_code     text NOT NULL REFERENCES weather_stations(code) ON DELETE CASCADE,
    year             int NOT NULL,
    month            int NOT NULL CHECK (month BETWEEN 1 AND 12),
    avg_daily_kwh_m2 double precision NOT NULL,
    valid_days       int NOT NULL,
    PRIMARY KEY (station_code, year, month)
);

-- Append-only, like pluvial_economy.
CREATE TABLE solar_economy (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endereco_id   uuid NOT NULL REFERENCES enderecos(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL,
    panel_power_w double precision NOT NULL,
    panel_count   int NOT NULL,
    system_kwp    double precision NOT NULL,
    loss_factor   double precision NOT NULL,
    station_code  text REFERENCES weather_stations(code),
    distance_km   double precision,
    period_start  date NOT NULL,
    period_end    date NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_solar_economy_endereco_created_at ON solar_economy(endereco_id, created_at DESC);

CREATE TABLE solar_monthly_generation (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    solar_economy_id uuid NOT NULL REFERENCES solar_economy(id) ON DELETE CASCADE,
    year             int NOT NULL,
    month            int NOT NULL CHECK (month BETWEEN 1 AND 12),
    avg_daily_kwh_m2 double precision NOT NULL,
    generated_kwh    double precision NOT NULL,
    UNIQUE (solar_economy_id, year, month)
);
