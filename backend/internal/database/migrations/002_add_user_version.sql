-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN version BIGINT NOT NULL DEFAULT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN version;
-- +goose StatementEnd