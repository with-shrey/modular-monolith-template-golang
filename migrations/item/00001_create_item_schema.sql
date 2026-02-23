-- +goose Up
CREATE SCHEMA IF NOT EXISTS item;

-- +goose Down
DROP SCHEMA IF EXISTS item;
