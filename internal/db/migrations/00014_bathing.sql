-- +goose Up

-- EEA bathing-water data (WISE BWD via Discodata, CC BY 4.0). Replaced whole
-- on each import; see internal/upstream/bathing/README.md.

CREATE TABLE bathing_site (
    site_id     text PRIMARY KEY CHECK (site_id ~ '^[A-Z]{2}[A-Za-z0-9]{1,40}$'),
    name_bg     text NOT NULL,
    name_en     text NOT NULL,
    zone        text NOT NULL CHECK (zone IN ('coastal', 'lake')),
    lat         double precision NOT NULL CHECK (lat BETWEEN -90 AND 90),
    lon         double precision NOT NULL CHECK (lon BETWEEN -180 AND 180),
    profile_url text NOT NULL
);

-- quality 'good_or_sufficient' is the pre-2015 transitional class.
CREATE TABLE bathing_class (
    site_id text NOT NULL REFERENCES bathing_site ON DELETE CASCADE,
    season  smallint NOT NULL CHECK (season BETWEEN 1990 AND 2100),
    quality text NOT NULL CHECK (quality IN
        ('not_classified', 'excellent', 'good', 'sufficient', 'poor', 'good_or_sufficient')),
    PRIMARY KEY (site_id, season)
);

-- Values in cfu/100 ml. *_below_detection: the lab reported "< value".
CREATE TABLE bathing_sample (
    site_id               text NOT NULL REFERENCES bathing_site ON DELETE CASCADE,
    sample_date           date NOT NULL,
    season                smallint NOT NULL CHECK (season BETWEEN 1990 AND 2100),
    ec_value              integer NOT NULL CHECK (ec_value >= 0),
    ec_below_detection    boolean NOT NULL,
    ie_value              integer NOT NULL CHECK (ie_value >= 0),
    ie_below_detection    boolean NOT NULL,
    pre_season            boolean NOT NULL,
    PRIMARY KEY (site_id, sample_date)
);

-- One row per successful import; the loop waits refresh_interval from the newest.
CREATE TABLE bathing_import (
    imported_at timestamptz PRIMARY KEY,
    sites       integer NOT NULL,
    classes     integer NOT NULL,
    samples     integer NOT NULL
);

-- +goose Down
DROP TABLE bathing_import;
DROP TABLE bathing_sample;
DROP TABLE bathing_class;
DROP TABLE bathing_site;
