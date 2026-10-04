package db

import "context"

func (q *Queries) ResetTimes(ctx context.Context, user, since int64) ([]int64, error) {
	rows, err := q.db.QueryContext(ctx, "SELECT occurred_at FROM device_resets WHERE user_id=$1 AND occurred_at>$2 ORDER BY occurred_at", user, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (q *Queries) RecordReset(ctx context.Context, user, at int64) error {
	_, err := q.db.ExecContext(ctx, "INSERT INTO device_resets(user_id,occurred_at) VALUES($1,$2)", user, at)
	return err
}
