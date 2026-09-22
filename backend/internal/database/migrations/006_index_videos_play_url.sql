-- +goose Up
-- +goose StatementBegin
-- 公开文件路由 /videos/{authorID}/{file} 要按请求路径反查视频,才知道该不该放行
-- (草稿只给作者看、已下架谁都不给)。请求路径和 videos.play_url 存的是同一个字符串,
-- 所以这里建索引让反查是索引命中而不是全表扫。
--
-- UNIQUE 不只是提速:它把「两个视频指向同一个文件」变成数据库层面不可能。
-- 文件名是 时间戳-uuid.ext,冲突概率可忽略,这个约束不会误伤
CREATE UNIQUE INDEX idx_videos_play_url ON videos(play_url);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_videos_play_url;
-- +goose StatementEnd
