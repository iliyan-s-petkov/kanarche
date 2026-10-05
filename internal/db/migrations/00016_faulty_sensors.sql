-- +goose Up

-- Per sensor-hour-metric reading counts, written by the rollup, and the faulty
-- set derived from them once per ingest cycle. See README.md.

CREATE TABLE reading_quality_hourly (
    bucket    timestamptz NOT NULL,
    sensor_id bigint NOT NULL,
    metric    text NOT NULL,
    total     integer NOT NULL CHECK (total > 0),
    flagged   integer NOT NULL CHECK (flagged >= 0 AND flagged <= total)
);

SELECT create_hypertable('reading_quality_hourly', 'bucket', chunk_time_interval => interval '1 day');

CREATE UNIQUE INDEX reading_quality_hourly_key_idx
    ON reading_quality_hourly (sensor_id, metric, bucket);

SELECT add_retention_policy('reading_quality_hourly', drop_after => interval '32 days');

CREATE TABLE sensor_faulty (
    sensor_id bigint NOT NULL,
    metric    text NOT NULL,
    PRIMARY KEY (sensor_id, metric)
);

-- +goose Down
DROP TABLE sensor_faulty;
SELECT remove_retention_policy('reading_quality_hourly');
DROP TABLE reading_quality_hourly;
