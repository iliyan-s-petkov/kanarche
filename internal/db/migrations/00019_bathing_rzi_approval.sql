-- +goose Up

-- A needs_review file can be approved by an operator. The approval is stored on
-- the file row: when, and how many rows were left out. record_sha256 is the
-- hash of the outbox record the verdict came from, so approval can check that
-- the review copy was not edited.
ALTER TABLE bathing_rzi_file
    ADD COLUMN approved_at   timestamptz,
    ADD COLUMN excluded_rows integer NOT NULL DEFAULT 0 CHECK (excluded_rows >= 0),
    ADD COLUMN record_sha256 text CHECK (record_sha256 IS NULL OR record_sha256 ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT bathing_rzi_file_approved_ok CHECK (approved_at IS NULL OR status = 'ok');

-- +goose Down
ALTER TABLE bathing_rzi_file
    DROP CONSTRAINT bathing_rzi_file_approved_ok,
    DROP COLUMN record_sha256,
    DROP COLUMN excluded_rows,
    DROP COLUMN approved_at;
