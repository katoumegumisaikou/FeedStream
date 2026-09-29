-- +goose Up
-- +goose StatementBegin
CREATE TABLE video_likes (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT    NOT NULL,
    video_id   BIGINT    NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_video_likes_user_video UNIQUE (user_id, video_id)
);

-- 按视频查询点赞记录/统计点赞数,并按点赞时间倒序排列。
CREATE INDEX idx_video_likes_video_time ON video_likes(video_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS video_likes;
-- +goose StatementEnd
