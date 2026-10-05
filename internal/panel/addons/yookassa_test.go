package addons

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNativeYooKassaModesAndVerifiedStatus(t *testing.T) {
	var creates int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, secret, ok := r.BasicAuth()
		test := user == "111"
		if !ok || !((test && secret == "test-secret") || (user == "222" && secret == "live-secret")) {
			t.Error("incorrect shop authentication")
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/me" {
			fmt.Fprintf(w, `{"account_id":%q,"test":%v}`, user, test)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/payments" {
			if r.Header.Get("Idempotence-Key") != "mirai-1" {
				t.Error("missing idempotency key")
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["capture"] != true {
				t.Error("payment is not captured automatically")
			}
			creates++
			fmt.Fprintf(w, `{"id":"invoice","test":%v,"confirmation":{"confirmation_url":"https://yoomoney.ru/pay"}}`, test)
			return
		}
		if r.URL.Path == "/payments/invoice" {
			fmt.Fprintf(w, `{"id":"invoice","test":%v,"paid":true,"status":"succeeded","amount":{"value":"199.90","currency":"RUB"}}`, test)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client := &Client{base: server.URL, hc: server.Client(), yoo: true}
	settings := Settings{"shop_id": "222", "secret_key": "live-secret", "test_shop_id": "111", "test_secret_key": "test-secret", "test_mode": true}
	ctx := context.Background()
	if err := client.Check(ctx, settings); err != nil {
		t.Fatal(err)
	}
	inv, err := client.CreateInvoice(ctx, InvoiceRequest{Settings: settings, PaymentID: 1, IdempotencyKey: "mirai-1", Amount: 19990, Currency: "RUB", ReturnURL: "https://t.me/test"})
	if err != nil || inv.ExternalID != "test:invoice" || creates != 1 {
		t.Fatal(inv, err)
	}
	settings["test_mode"] = false
	status, err := client.Status(ctx, settings, inv.ExternalID)
	if err != nil || status.Status != "paid" || status.Amount != 19990 {
		t.Fatal("test invoice lost after switching to live", status, err)
	}
	id, err := client.Webhook(ctx, settings, "", nil, []byte(`{"type":"notification","event":"payment.succeeded","object":{"id":"invoice","test":true}}`))
	if err != nil || id != inv.ExternalID {
		t.Fatal(id, err)
	}
	settings["shop_id"] = "111"
	settings["secret_key"] = "test-secret"
	if err := client.Check(ctx, settings); err == nil {
		t.Fatal("test shop accepted as a live shop")
	}
	if strings.Contains(fmt.Sprint(YooKassaInfo()), "test-secret") {
		t.Fatal("secret exposed")
	}
}
