-- +goose Up
-- +goose StatementBegin
CREATE TABLE play_records (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    video_id   BIGINT      NOT NULL,
    author_id  BIGINT      NOT NULL,
    watched    INTEGER     NOT NULL DEFAULT 0,
    duration   INTEGER     NOT NULL DEFAULT 0,
    ip         VARCHAR(45),
    created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 某人的观看历史:user_id 等值 + 时间倒序,列顺序与排序方向和查询完全对应
CREATE INDEX idx_play_user_time ON play_records(user_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS play_records;
-- +goose StatementEnd
