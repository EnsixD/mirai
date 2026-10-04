package domain

import (
	"context"
	"mirai/internal/panel/store"
	"mirai/internal/panel/store/db"
)

var ErrTariffPayments = db.ErrTariffUnsettled

func DeleteTariff(ctx context.Context, st *store.Store, id int64) error {
	return st.Tx(ctx, func(q *db.Queries) error { return q.DeleteTariffPermanent(ctx, id) })
}
