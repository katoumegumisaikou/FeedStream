-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    user_name     VARCHAR(64)  NOT NULL,
    password      VARCHAR(16)  NOT NULL,
    phone         VARCHAR(20)  NOT NULL UNIQUE,
    email         VARCHAR(128) UNIQUE,
    avatar_url    VARCHAR(512),
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    deleted_at    TIMESTAMP
);

-- 软删除字段加索引(常见查询条件)
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

-- 按注册时间排序/筛选常见,加索引
CREATE INDEX idx_users_created_at ON users(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
