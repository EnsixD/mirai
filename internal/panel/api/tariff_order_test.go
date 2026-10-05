package api

import (
	"context"
	"mirai/internal/panel/store/storetest"
	"testing"
)

func TestTariffOrderPersistsAndFiltersSales(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = st.DB.ExecContext(ctx, "INSERT INTO tariffs (id,name,on_sale,duration_days,created_at) VALUES (1,'A',1,30,1),(2,'B',0,30,1),(3,'C',1,30,1)")
	if err != nil {
		t.Fatal(err)
	}
	h := &handlers{d: Deps{Store: st}}
	in := &tariffOrderInput{}
	in.Body.IDs = []int64{3, 2, 1}
	if _, err = h.orderTariffs(ctx, in); err != nil {
		t.Fatal(err)
	}
	all, _ := st.Q.ListTariffs(ctx)
	sale, _ := st.Q.ListTariffsOnSale(ctx)
	if len(all) != 3 || all[0].ID != 3 || all[1].ID != 2 || len(sale) != 2 || sale[0].ID != 3 || sale[1].ID != 1 {
		t.Fatalf("wrong order: %v %v", all, sale)
	}
	in.Body.IDs = []int64{1, 1, 3}
	if _, err = h.orderTariffs(ctx, in); err == nil {
		t.Fatal("duplicate accepted")
	}
	after, _ := st.Q.ListTariffs(ctx)
	if after[0].ID != 3 {
		t.Fatal("invalid order changed data")
	}
}
