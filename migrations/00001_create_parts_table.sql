-- +goose Up
CREATE TABLE IF NOT EXISTS parts (
    id UUID PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    category VARCHAR(100) NOT NULL,
    current_stock BIGINT NOT NULL,
    minimum_stock BIGINT NOT NULL CHECK (minimum_stock >= 0),
    average_daily_sales NUMERIC NOT NULL CHECK (average_daily_sales >= 0),
    lead_time_days INT NOT NULL CHECK (lead_time_days >= 0),
    unit_cost NUMERIC(15, 2) NOT NULL CHECK (unit_cost >= 0),
    criticality_level INT NOT NULL CHECK (criticality_level BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_parts_category ON parts (category);

-- +goose Down
DROP TABLE IF EXISTS parts;
