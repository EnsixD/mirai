-- +goose Up
UPDATE bound_devices SET created_at=created_at/1000 WHERE hwid LIKE 'legacy-sha256:%' AND created_at > 253402300799;
UPDATE bound_devices SET last_seen=last_seen/1000 WHERE hwid LIKE 'legacy-sha256:%' AND last_seen > 253402300799;
-- +goose Down
-- Normalized imported timestamps remain valid seconds on rollback.
SELECT 1;
