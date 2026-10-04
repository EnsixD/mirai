package db

import (
	"context"
	"database/sql"
	"errors"
)

var ErrTariffUnsettled = errors.New("tariff has unsettled payments")

func (q *Queries) DeleteTariffPermanent(ctx context.Context, id int64) error {
	var days int64
	if err := q.db.QueryRowContext(ctx, "SELECT duration_days FROM tariffs WHERE id=$1 FOR UPDATE", id).Scan(&days); err != nil {
		return err
	}
	var unsettled bool
	if err := q.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM payments WHERE tariff_id=$1 AND status IN ('pending','paid'))", id).Scan(&unsettled); err != nil {
		return err
	}
	if unsettled {
		return ErrTariffUnsettled
	}
	if _, err := q.db.ExecContext(ctx, "UPDATE payments SET term_days=COALESCE(term_days,$2) WHERE tariff_id=$1", id, days); err != nil {
		return err
	}
	result, err := q.db.ExecContext(ctx, "DELETE FROM tariffs WHERE id=$1", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
