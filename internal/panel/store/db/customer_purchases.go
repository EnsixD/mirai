package db

import "context"

type CustomerPurchases struct{ RublesKopecks, Days, Purchases, Unlimited int64 }

func (q *Queries) CustomerPurchases(ctx context.Context, chat int64) (CustomerPurchases, error) {
	var result CustomerPurchases
	err := q.db.QueryRowContext(ctx, `SELECT
 COALESCE(SUM(p.amount) FILTER (WHERE p.currency='RUB'),0),
 COALESCE(SUM(COALESCE(p.term_days,t.duration_days,0)) FILTER (WHERE p.kind IN ('new','renew')),0),
 COUNT(*) FILTER (WHERE p.kind IN ('new','renew')),
 COUNT(*) FILTER (WHERE p.kind IN ('new','renew') AND COALESCE(p.term_days,t.duration_days,-1)=0)
 FROM payments p LEFT JOIN tariffs t ON t.id=p.tariff_id
 WHERE p.tg_id=$1 AND p.status='applied' AND COALESCE(p.external_id,'') NOT LIKE 'test:%'`, chat).Scan(&result.RublesKopecks, &result.Days, &result.Purchases, &result.Unlimited)
	return result, err
}
