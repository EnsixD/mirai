package db

import "context"

type DeviceTraffic struct{ Up, Down int64 }

// CountDeviceTraffic runs on the same transaction as the batch acknowledgement,
// so retries cannot count the same node batch twice.
func (q *Queries) CountDeviceTraffic(ctx context.Context, slot string, up, down int64) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO bound_device_traffic(device_id, up, down)
SELECT d.id, $2, $3 FROM bound_devices d JOIN slots s ON s.id=d.slot_id WHERE s.name=$1
ON CONFLICT(device_id) DO UPDATE SET up=bound_device_traffic.up+excluded.up, down=bound_device_traffic.down+excluded.down`, slot, up, down)
	return err
}

func (q *Queries) DeviceTrafficOf(ctx context.Context, user int64) (map[int64]DeviceTraffic, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT d.id, COALESCE(t.up,0), COALESCE(t.down,0) FROM bound_devices d LEFT JOIN bound_device_traffic t ON t.device_id=d.id WHERE d.user_id=$1`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]DeviceTraffic{}
	for rows.Next() {
		var id int64
		var t DeviceTraffic
		if err := rows.Scan(&id, &t.Up, &t.Down); err != nil {
			return nil, err
		}
		out[id] = t
	}
	return out, rows.Err()
}
