-- +goose Up
-- The list endpoint searches with ILIKE '%term%'. A leading wildcard cannot use
-- a B-tree index, so without a trigram index every search is a sequential scan
-- of the whole table. pg_trgm turns the substring match into an index lookup.

-- A managed database may forbid CREATE EXTENSION. Failing the migration there
-- would block the deployment on an optimisation rather than on a defect, so the
-- extension is attempted and its absence is reported instead of fatal.
--
-- StatementBegin/End are required: goose splits the file on semicolons, and the
-- body of a DO block is full of them.
-- +goose StatementBegin
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_trgm;
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'pg_trgm is unavailable; note search falls back to a sequential scan';
END $$;
-- +goose StatementEnd

-- title and content are searched together, so both get a GIN index over
-- gin_trgm_ops. Guarded by IF NOT EXISTS so a re-run is harmless, and skipped
-- entirely when the extension could not be installed.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_notes_title_trgm ON notes USING gin (title gin_trgm_ops)';
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_notes_content_trgm ON notes USING gin (content gin_trgm_ops)';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    EXECUTE 'DROP INDEX IF EXISTS idx_notes_title_trgm';
    EXECUTE 'DROP INDEX IF EXISTS idx_notes_content_trgm';
END $$;
-- +goose StatementEnd
