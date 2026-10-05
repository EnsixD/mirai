-- +goose Up
CREATE TABLE tg_account_controls (
  tg_id BIGINT PRIMARY KEY REFERENCES tg_chats(tg_id) ON DELETE CASCADE,
  banned BOOLEAN NOT NULL DEFAULT FALSE,
  resume_ids BIGINT[] NOT NULL DEFAULT '{}'
);
-- New subscriptions issued while an account is banned remain disabled.
-- +goose StatementBegin
CREATE FUNCTION tg_banned_link() RETURNS trigger AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM tg_account_controls WHERE tg_id=NEW.tg_id AND banned) THEN
    UPDATE tg_account_controls SET resume_ids=array_append(resume_ids,NEW.user_id)
      WHERE tg_id=NEW.tg_id AND EXISTS (SELECT 1 FROM users WHERE id=NEW.user_id AND status<>'disabled');
    UPDATE users SET status='disabled' WHERE id=NEW.user_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER tg_banned_link AFTER INSERT OR UPDATE ON tg_links FOR EACH ROW EXECUTE FUNCTION tg_banned_link();
-- +goose StatementBegin
CREATE FUNCTION tg_banned_user() RETURNS trigger AS $$
BEGIN
  IF NEW.status<>'disabled' AND EXISTS (SELECT 1 FROM tg_links l JOIN tg_account_controls c ON c.tg_id=l.tg_id WHERE l.user_id=NEW.id AND c.banned) THEN
    NEW.status='disabled';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER tg_banned_user BEFORE UPDATE OF status ON users FOR EACH ROW EXECUTE FUNCTION tg_banned_user();

-- +goose Down
DROP TRIGGER tg_banned_user ON users;
DROP FUNCTION tg_banned_user();
DROP TRIGGER tg_banned_link ON tg_links;
DROP FUNCTION tg_banned_link();
DROP TABLE tg_account_controls;
