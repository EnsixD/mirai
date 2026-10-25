package db

import "context"

type TelegramAccount struct {
	ID, UserID, CreatedAt int64
	Username, Name        string
}

func (q *Queries) CountTelegramVisitors(ctx context.Context) (int64, error) {
	var count int64
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tg_chats c WHERE NOT EXISTS (SELECT 1 FROM tg_links l WHERE l.tg_id=c.tg_id)`).Scan(&count)
	return count, err
}

func (q *Queries) BoundDeviceCounts(ctx context.Context) (map[int64]int64, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT user_id, COUNT(*) FROM bound_devices GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[int64]int64{}
	for rows.Next() {
		var user, count int64
		if err := rows.Scan(&user, &count); err != nil {
			return nil, err
		}
		counts[user] = count
	}
	return counts, rows.Err()
}

func (q *Queries) TelegramAccounts(ctx context.Context) ([]TelegramAccount, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT c.tg_id,COALESCE(l.user_id,0),c.created_at,c.username,c.first_name FROM tg_chats c LEFT JOIN tg_links l ON l.tg_id=c.tg_id ORDER BY c.created_at DESC,c.tg_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TelegramAccount{}
	for rows.Next() {
		var a TelegramAccount
		if err := rows.Scan(&a.ID, &a.UserID, &a.CreatedAt, &a.Username, &a.Name); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
