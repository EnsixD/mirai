package db

import "context"

func (q *Queries) SetTariffSort(ctx context.Context, id, position int64) error {
	_, err := q.db.ExecContext(ctx, "UPDATE tariffs SET sort=$2 WHERE id=$1 AND archived=0", id, position)
	return err
}
