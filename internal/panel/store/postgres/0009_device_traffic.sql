-- +goose Up
CREATE TABLE bound_device_traffic (
  device_id BIGINT PRIMARY KEY REFERENCES bound_devices(id) ON DELETE CASCADE,
  up BIGINT NOT NULL DEFAULT 0 CHECK (up >= 0),
  down BIGINT NOT NULL DEFAULT 0 CHECK (down >= 0)
);

-- +goose Down
DROP TABLE bound_device_traffic;
