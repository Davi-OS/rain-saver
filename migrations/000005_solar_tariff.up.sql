-- Electricity price (R$/kWh) the user entered for this calculation; NULL when
-- they only wanted the kWh estimate. Savings are derived from it on read.
ALTER TABLE solar_economy ADD COLUMN tariff_brl_per_kwh double precision;
