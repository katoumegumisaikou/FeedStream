-- +goose Up
-- +goose StatementBegin
-- 新插入的视频默认状态应该是「草稿」(4),而不是 0「转码中」——
-- 本项目没有接入转码,0 是一个永远不会被写入的状态。
-- 分片上传完成后建的行会显式写 status,这个默认值只是防止有人漏写。
ALTER TABLE videos ALTER COLUMN status SET DEFAULT 4;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE videos ALTER COLUMN status SET DEFAULT 0;
-- +goose StatementEnd
