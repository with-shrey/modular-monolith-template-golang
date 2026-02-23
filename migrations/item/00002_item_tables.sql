-- +goose Up
CREATE TABLE IF NOT EXISTS item.items (
    id         UUID      PRIMARY KEY,
    org_id     UUID      NOT NULL,
    name       TEXT      NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_items_org_id ON item.items (org_id);

-- +goose Down
DROP TABLE IF EXISTS item.items;
