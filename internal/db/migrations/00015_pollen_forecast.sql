-- +goose Up

-- Forecast pollen (CAMS via Open-Meteo) on a lattice of model cells. Model
-- output, not counts. lon_c/lat_c are centidegrees. See docs/pollen.md.

CREATE TABLE pollen_forecast (
    valid_at   timestamptz NOT NULL,
    lon_c      integer NOT NULL,
    lat_c      integer NOT NULL,
    species    text NOT NULL CHECK (species IN ('alder', 'birch', 'grass', 'mugwort', 'olive', 'ragweed')),
    grains     double precision NOT NULL CHECK (grains >= 0),
    fetched_at timestamptz NOT NULL
);

SELECT create_hypertable('pollen_forecast', 'valid_at', chunk_time_interval => interval '1 day');

CREATE UNIQUE INDEX pollen_forecast_key_idx
    ON pollen_forecast (lon_c, lat_c, species, valid_at DESC);

SELECT add_retention_policy('pollen_forecast', drop_after => interval '7 days');

-- +goose Down
SELECT remove_retention_policy('pollen_forecast');
DROP TABLE pollen_forecast;
