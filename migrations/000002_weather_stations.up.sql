-- weather_stations and station_monthly_rainfall are populated by a
-- separate INMET ingestion process (not part of this repo) — this service
-- only reads from them.
CREATE TABLE weather_stations (
    code       text PRIMARY KEY,
    name       text NOT NULL,
    latitude   double precision NOT NULL,
    longitude  double precision NOT NULL,
    uf         text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE station_monthly_rainfall (
    station_code text NOT NULL REFERENCES weather_stations(code) ON DELETE CASCADE,
    year         int NOT NULL,
    month        int NOT NULL CHECK (month BETWEEN 1 AND 12),
    rainfall_mm  double precision NOT NULL,
    PRIMARY KEY (station_code, year, month)
);

ALTER TABLE pluvial_economy
    ADD COLUMN station_code text REFERENCES weather_stations(code),
    ADD COLUMN distance_km  double precision;
