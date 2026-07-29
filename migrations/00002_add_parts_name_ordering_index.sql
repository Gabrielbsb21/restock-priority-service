-- +goose Up
-- The stable default ordering for GET /parts is LOWER(name) ASC, id ASC
-- (SPEC-001/BR-008). Without an expression index every page pays a full sort.
CREATE INDEX IF NOT EXISTS idx_parts_lower_name_id ON parts (LOWER(name), id);

-- +goose Down
DROP INDEX IF EXISTS idx_parts_lower_name_id;
