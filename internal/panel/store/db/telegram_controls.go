package db

import "context"

// DeleteTelegramVisitor removes only an account that still has no subscriptions.
func (q *Queries) DeleteTelegramVisitor(ctx context.Context, id int64) (int64, error) {
	r, err := q.db.ExecContext(ctx, `DELETE FROM tg_chats c WHERE c.tg_id=$1 AND NOT EXISTS (SELECT 1 FROM tg_links l WHERE l.tg_id=c.tg_id)`, id)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

type AdminBotStats struct{ Accounts, Banned, Keys, Orders, Pending int64 }

func (q *Queries) AdminTelegramStats(ctx context.Context) (AdminBotStats, error) {
	var s AdminBotStats
	err := q.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM tg_chats),(SELECT count(*) FROM tg_account_controls WHERE banned),(SELECT count(*) FROM users),(SELECT count(*) FROM payments),(SELECT count(*) FROM payments WHERE status='pending')`).Scan(&s.Accounts, &s.Banned, &s.Keys, &s.Orders, &s.Pending)
	return s, err
}

func (q *Queries) AccountBanned(ctx context.Context, id int64) (bool, error) {
	var banned bool
	err := q.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tg_account_controls WHERE tg_id=$1 AND banned)`, id).Scan(&banned)
	return banned, err
}

// ToggleAccountBan runs inside the caller's transaction; existing frozen keys stay frozen.
func (q *Queries) ToggleAccountBan(ctx context.Context, id int64) (bool, error) {
	if _, err := q.db.ExecContext(ctx, `INSERT INTO tg_account_controls(tg_id) VALUES($1) ON CONFLICT DO NOTHING`, id); err != nil {
		return false, err
	}
	var banned bool
	if err := q.db.QueryRowContext(ctx, `SELECT banned FROM tg_account_controls WHERE tg_id=$1 FOR UPDATE`, id).Scan(&banned); err != nil {
		return false, err
	}
	if banned {
		if _, err := q.db.ExecContext(ctx, `UPDATE tg_account_controls SET banned=false WHERE tg_id=$1`, id); err != nil {
			return false, err
		}
		if _, err := q.db.ExecContext(ctx, `UPDATE users SET status='active' WHERE id=ANY((SELECT resume_ids FROM tg_account_controls WHERE tg_id=$1)::bigint[]) AND status='disabled' AND EXISTS(SELECT 1 FROM tg_links WHERE user_id=users.id AND tg_id=$1)`, id); err != nil {
			return false, err
		}
		_, err := q.db.ExecContext(ctx, `UPDATE tg_account_controls SET banned=false,resume_ids='{}' WHERE tg_id=$1`, id)
		return false, err
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE tg_account_controls SET banned=true,resume_ids=ARRAY(SELECT u.id FROM users u JOIN tg_links l ON l.user_id=u.id WHERE l.tg_id=$1 AND u.status<>'disabled') WHERE tg_id=$1`, id); err != nil {
		return false, err
	}
	_, err := q.db.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id IN(SELECT user_id FROM tg_links WHERE tg_id=$1)`, id)
	return true, err
}

func (q *Queries) DeleteTelegramAccount(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM tg_chats WHERE tg_id=$1`, id)
	return err
}

func (q *Queries) ResetAccountTrial(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM trials WHERE tg_id=$1`, id)
	return err
}

func (q *Queries) AccountTrialUsed(ctx context.Context, id int64) (bool, error) {
	var used bool
	err := q.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM trials WHERE tg_id=$1)`, id).Scan(&used)
	return used, err
}
