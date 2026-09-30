-- +goose Up
-- +goose StatementBegin
-- 分页游标从「单个 created_at」改成「(created_at, id) 复合」,索引也要跟着覆盖 id:
-- 列顺序和方向必须和 ORDER BY created_at DESC, id DESC 逐列对应,否则索引只能吃到
-- 第一个键,后面按 id 的比较还要 PostgreSQL 自己算。
--
-- 直接替换而不是新增:复合索引的前缀就是原来的单列索引,留着旧的只是多一份写放大,
-- 而且规划器不会再选它
DROP INDEX idx_videos_create_time;
CREATE INDEX idx_videos_create_time_id ON videos(created_at DESC, id DESC);

-- 这个索引的名字里一直有 _id,但列定义里只有 likes_count —— 现在名副其实了。
-- 点赞数重复很常见(0 赞的视频一大把),没有 id 兜底的话分页会漏条
DROP INDEX idx_videos_likes_count_id;
CREATE INDEX idx_videos_likes_count_id ON videos(likes_count DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_videos_create_time_id;
CREATE INDEX idx_videos_create_time ON videos(created_at DESC);

DROP INDEX idx_videos_likes_count_id;
CREATE INDEX idx_videos_likes_count_id ON videos(likes_count DESC);
-- +goose StatementEnd
