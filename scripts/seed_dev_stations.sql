-- Dev-only fixture: NOT a migration, not run in production. Real station
-- and rainfall data is populated by a separate INMET ingestion process
-- (not part of this repo yet). This just seeds two real INMET stations
-- with synthetic monthly rainfall, in the exact shape the ingestion side
-- will eventually produce, so the read path can be exercised locally
-- without waiting for that ingestion to exist.
--
-- Usage: psql "postgres://rainsaver:rainsaver@localhost:5433/rainsaver?sslmode=disable" -f scripts/seed_dev_stations.sql

INSERT INTO weather_stations (code, name, latitude, longitude, uf) VALUES
    ('A701', 'SAO PAULO - MIRANTE', -23.4977, -46.6202, 'SP'),
    ('A652', 'BELO HORIZONTE - CERCADINHO', -19.9392, -43.9386, 'MG')
ON CONFLICT (code) DO NOTHING;

-- Synthetic monthly totals (mm), roughly following each region's wet/dry
-- season shape — illustrative only, not real INMET measurements.
INSERT INTO station_monthly_rainfall (station_code, year, month, rainfall_mm) VALUES
    ('A701', 2025, 9, 65), ('A701', 2025, 10, 130), ('A701', 2025, 11, 125), ('A701', 2025, 12, 180),
    ('A701', 2026, 1, 220), ('A701', 2026, 2, 195), ('A701', 2026, 3, 140), ('A701', 2026, 4, 65),
    ('A701', 2026, 5, 70),  ('A701', 2026, 6, 45),  ('A701', 2026, 7, 30),  ('A701', 2026, 8, 35),

    ('A652', 2025, 9, 30),  ('A652', 2025, 10, 95),  ('A652', 2025, 11, 180), ('A652', 2025, 12, 260),
    ('A652', 2026, 1, 230), ('A652', 2026, 2, 170),  ('A652', 2026, 3, 140),  ('A652', 2026, 4, 40),
    ('A652', 2026, 5, 20),  ('A652', 2026, 6, 10),   ('A652', 2026, 7, 5),    ('A652', 2026, 8, 15)
ON CONFLICT (station_code, year, month) DO NOTHING;
