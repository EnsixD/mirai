package domain

import (
	"context"
	"testing"
	"time"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/storetest"
)

// Empty catalogs remain empty across repeated starts, regardless of language.
func TestSeedLeavesCatalogEmpty(t *testing.T) {
	for _, lang := range []string{"", "ru", "en"} {
		ctx := context.Background()
		st, err := storetest.Open(ctx, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if lang != "" {
			if err := settings.Set(ctx, settings.New(st.Q), settings.KeyDefaultLang, lang); err != nil {
				t.Fatal(err)
			}
		}
		if err := Seed(ctx, st, time.Now()); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			inbounds, err := st.Q.ListInbounds(ctx)
			if err != nil || len(inbounds) != 0 {
				t.Fatalf("fresh install must have no inbounds: %d %v", len(inbounds), err)
			}
			if err := Seed(ctx, st, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
		tariffs, err := st.Q.ListTariffs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(tariffs) != 0 {
			t.Fatalf("seed created %d tariffs", len(tariffs))
		}
		st.Close()
	}
}
