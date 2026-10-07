-- +goose Up

-- Where a class came from, so the API can say "from the EEA annual report".
-- Additive: a constant default is a catalog change in Postgres 11+, and the
-- tables hold a few thousand rows, so the CHECK scan is instant.
ALTER TABLE bathing_class
    ADD COLUMN source text NOT NULL DEFAULT 'discodata'
    CHECK (source IN ('discodata', 'datahub'));

-- Edition of the embedded snapshot used by the import, NULL when none was.
ALTER TABLE bathing_import ADD COLUMN supplement_edition text;

-- +goose Down
-- Rows only the snapshot could supply would read as Discodata once the column
-- is gone. The next import restores them.
DELETE FROM bathing_class WHERE source = 'datahub';
ALTER TABLE bathing_import DROP COLUMN supplement_edition;
ALTER TABLE bathing_class DROP COLUMN source;
