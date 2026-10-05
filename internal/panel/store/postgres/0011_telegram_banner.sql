-- +goose Up
ALTER TABLE tg_chats ADD COLUMN banner_msg_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tg_chats ADD COLUMN banner_file_id TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE tg_chats DROP COLUMN banner_msg_id;
ALTER TABLE tg_chats DROP COLUMN banner_file_id;
