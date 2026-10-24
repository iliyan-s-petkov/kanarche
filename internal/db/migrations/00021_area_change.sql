-- +goose Up
CREATE TABLE area_change (
    slug        text PRIMARY KEY,
    fingerprint text        NOT NULL,
    changed_at  timestamptz NOT NULL
);

-- +goose Down
DROP TABLE area_change;
