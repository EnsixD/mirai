package billing

import (
	"context"
	"testing"
)

func TestMiraiDisablesNewStarsPayments(t *testing.T) {
	e := newEnv(t) // fixture deliberately has a saved enabled Stars configuration.
	ctx := context.Background()
	c, err := e.s.LoadConfig(ctx)
	if err != nil || c.Stars {
		t.Fatalf("saved Stars settings must be disabled: %+v %v", c, err)
	}
	if e.s.Available(ctx).Stars {
		t.Fatal("Stars still offered to customers")
	}
	offers, _, err := e.s.Offers(ctx)
	if err != nil || len(offers) != 0 {
		t.Fatalf("no YooKassa adapter means no payment offers: %d %v", len(offers), err)
	}
}
