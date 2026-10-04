-- +goose Up
ALTER TABLE payments DROP CONSTRAINT payments_check;
ALTER TABLE payments DROP CONSTRAINT payments_tariff_id_fkey;
ALTER TABLE payments ADD CONSTRAINT payments_tariff_id_fkey FOREIGN KEY (tariff_id) REFERENCES tariffs(id) ON DELETE SET NULL;
ALTER TABLE payments ADD CONSTRAINT payments_tariff_history_check CHECK (kind = 'package' OR tariff_id IS NOT NULL OR tariff_name <> '');

-- +goose Down
ALTER TABLE payments DROP CONSTRAINT payments_tariff_history_check;
ALTER TABLE payments DROP CONSTRAINT payments_tariff_id_fkey;
ALTER TABLE payments ADD CONSTRAINT payments_tariff_id_fkey FOREIGN KEY (tariff_id) REFERENCES tariffs(id);
ALTER TABLE payments ADD CONSTRAINT payments_check CHECK (kind = 'package' OR tariff_id IS NOT NULL) NOT VALID;
