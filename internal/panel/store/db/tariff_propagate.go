package db

import "context"

// Refresh tariff-derived limits without changing paid time or resetting used traffic.
func (q *Queries) PropagateTariff(ctx context.Context, tariff Tariff, now int64) ([]int64, error) {
	rows, err := q.db.QueryContext(ctx, `UPDATE users SET traffic_limit=$2,device_limit=$3,reset_strategy=$4,billing_day=$5,updated_at=$6 WHERE tariff_id=$1 RETURNING id`, tariff.ID, tariff.TrafficLimit, tariff.DeviceLimit, tariff.ResetStrategy, tariff.BillingDay, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
