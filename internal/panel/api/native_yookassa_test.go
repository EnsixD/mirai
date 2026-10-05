package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mirai/internal/panel/addons"
	"mirai/internal/panel/billing"
	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/storetest"
	"strings"
	"testing"
	"time"
)

func TestNativeYooKassaSettingsWithoutMarketplaceAndSecretExposure(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := addons.New("", "http://127.0.0.1:1/unavailable", "0.5.0.10", log, time.Now)
	manager.EnableYooKassa()
	set := settings.New(st.Q)
	service := billing.New(billing.Deps{Store: st, Settings: set, Addons: manager, Log: log, Now: time.Now})
	_, err = service.SetAddonConfig(ctx, "yookassa", false, addons.Settings{"shop_id": "222", "secret_key": "live-secret", "test_shop_id": "111", "test_secret_key": "test-secret", "test_mode": true})
	if err != nil {
		t.Fatal(err)
	}
	h := handlers{d: Deps{Store: st, Settings: set, Billing: service, Addons: manager, Log: log, Now: time.Now}}
	view, err := h.addonsView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.CatalogError != "" || !view.Supported || len(view.Installed) != 1 || view.Installed[0].Version != "native" {
		t.Fatal("native provider depends on marketplace", view)
	}
	data, _ := json.Marshal(view)
	if strings.Contains(string(data), "live-secret") || strings.Contains(string(data), "test-secret") {
		t.Fatal("API exposed secret shop credentials")
	}
	for _, field := range view.Installed[0].Settings {
		if field.Secret && (!field.Set || field.Value != nil) {
			t.Fatal("saved secret state is wrong", field.Key)
		}
	}
}
