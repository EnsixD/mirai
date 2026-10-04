package db

import "context"

type TelegramAccount struct {
	ID, UserID, CreatedAt int64
	Username, Name        string
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
