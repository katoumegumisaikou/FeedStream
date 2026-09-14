-- +goose Up
-- +goose StatementBegin
CREATE TABLE videos (
    id            BIGSERIAL PRIMARY KEY,
    author_id     BIGINT        NOT NULL,
    username      VARCHAR(255)  NOT NULL,
    avatar_url    VARCHAR(512),
    title         VARCHAR(255)  NOT NULL,
    description   VARCHAR(1000),
    play_url      VARCHAR(255)  NOT NULL,
    cover_url     VARCHAR(255)  NOT NULL,
    created_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMP,
    status        SMALLINT      NOT NULL DEFAULT 0,
    play_count    BIGINT        NOT NULL DEFAULT 0,
    likes_count   BIGINT        NOT NULL DEFAULT 0,
    comment_count BIGINT        NOT NULL DEFAULT 0,
    popularity    BIGINT        NOT NULL DEFAULT 0
);

-- 某人的视频列表:author_id 等值查询
CREATE INDEX idx_videos_author_id ON videos(author_id);

-- 全局最新流:按发布时间倒序
CREATE INDEX idx_videos_create_time ON videos(created_at DESC);

-- 热门流:热度降序,同热度再按时间降序(复合索引列顺序对应 priority)
CREATE INDEX idx_videos_popularity_time_id ON videos(popularity DESC, created_at DESC);

-- 点赞排行榜
CREATE INDEX idx_videos_likes_count_id ON videos(likes_count DESC);

-- 按状态筛选(0 转码中 / 1 已发布 / 2 转码失败 / 3 已下架)
CREATE INDEX idx_videos_status ON videos(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS videos;
-- +goose StatementEnd
