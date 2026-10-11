-- +goose Up

-- Current-season RZI results, kept apart from the EEA tables so the weekly EEA
-- replace cannot touch them. One row per fetched file; the sha256 is its identity.
CREATE TABLE bathing_rzi_file (
    id                bigserial PRIMARY KEY,
    source            text NOT NULL CHECK (source IN ('rzi_varna', 'rzi_burgas', 'rzi_dobrich')),
    url               text NOT NULL,
    sha256            text NOT NULL UNIQUE CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    bytes             integer NOT NULL CHECK (bytes >= 0),
    etag              text NOT NULL DEFAULT '',
    last_modified     timestamptz,
    fetched_at        timestamptz NOT NULL,
    extractor_version text NOT NULL,
    status            text NOT NULL CHECK (status IN ('ok', 'needs_review', 'rejected')),
    rows_ok           integer NOT NULL CHECK (rows_ok >= 0),
    rows_rejected     integer NOT NULL CHECK (rows_rejected >= 0),
    reason            text NOT NULL DEFAULT ''
);

-- site_id is deliberately not a foreign key to bathing_site. The EEA replace
-- runs DELETE FROM bathing_site, which would cascade here or fail the delete.
-- The ingest validator maps points to EEA ids before anything is written.
CREATE TABLE bathing_rzi_sample (
    site_id     text NOT NULL,
    sample_date date NOT NULL,
    season      smallint NOT NULL CHECK (season BETWEEN 1990 AND 2100),
    ec_value    integer NOT NULL CHECK (ec_value BETWEEN 0 AND 1000000),
    ec_below    boolean NOT NULL,
    ie_value    integer NOT NULL CHECK (ie_value BETWEEN 0 AND 1000000),
    ie_below    boolean NOT NULL,
    unit        text NOT NULL CHECK (unit IN ('MPN/100ml', 'cfu/100ml')),
    remarks     text NOT NULL DEFAULT '' CHECK (char_length(remarks) <= 500),
    file_id     bigint NOT NULL REFERENCES bathing_rzi_file,
    PRIMARY KEY (site_id, sample_date)
);

CREATE INDEX bathing_rzi_sample_file_idx ON bathing_rzi_sample (file_id);

-- +goose Down
DROP TABLE bathing_rzi_sample;
DROP TABLE bathing_rzi_file;
