-- +goose Up

-- A needs_review file can be approved by an operator. The approval is stored on
-- the file row: when, and how many rows were left out.
ALTER TABLE bathing_rzi_file
    ADD COLUMN approved_at   timestamptz,
    ADD COLUMN excluded_rows integer NOT NULL DEFAULT 0 CHECK (excluded_rows >= 0),
    ADD CONSTRAINT bathing_rzi_file_approved_ok CHECK (approved_at IS NULL OR status = 'ok');

-- +goose Down
ALTER TABLE bathing_rzi_file
    DROP CONSTRAINT bathing_rzi_file_approved_ok,
    DROP COLUMN excluded_rows,
    DROP COLUMN approved_at;
