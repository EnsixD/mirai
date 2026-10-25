-- The 2trillion bot (internal/panel/ourbot): account bans and the menu banner.
-- No functions or triggers: a backup must hold tables only (cli/dumpcheck.go).
-- +goose Up
CREATE TABLE tg_account_controls (
  tg_id BIGINT PRIMARY KEY REFERENCES tg_chats(tg_id) ON DELETE CASCADE,
  banned BOOLEAN NOT NULL DEFAULT FALSE,
  resume_ids BIGINT[] NOT NULL DEFAULT '{}'
);
ALTER TABLE tg_chats ADD COLUMN IF NOT EXISTS banner_msg_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tg_chats ADD COLUMN IF NOT EXISTS banner_file_id TEXT NOT NULL DEFAULT '';
-- +goose Down
DROP TABLE IF EXISTS tg_account_controls;
ALTER TABLE tg_chats DROP COLUMN IF EXISTS banner_msg_id;
ALTER TABLE tg_chats DROP COLUMN IF EXISTS banner_file_id;
