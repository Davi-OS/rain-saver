ALTER TABLE pluvial_economy
    DROP COLUMN IF EXISTS station_code,
    DROP COLUMN IF EXISTS distance_km;

DROP TABLE IF EXISTS station_monthly_rainfall;
DROP TABLE IF EXISTS weather_stations;
