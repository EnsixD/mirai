package billing

import (
	"context"
	"fmt"
	"io"
	"mirai/internal/panel/addons"
	"net/http"
	"strings"
	"testing"
)

type yooTransport func(*http.Request) (*http.Response, error)

func (f yooTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNativeYooKassaAutomaticIssueAndModeSwitch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	previous := http.DefaultTransport
	defer func() { http.DefaultTransport = previous }()
	http.DefaultTransport = yooTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.yookassa.ru" {
			return previous.RoundTrip(r)
		}
		shop, key, ok := r.BasicAuth()
		test := shop == "111"
		if !ok || !((test && key == "test-secret") || (shop == "222" && key == "live-secret")) {
			t.Fatal("wrong credentials")
		}
		body := ""
		switch {
		case r.URL.Path == "/v3/me":
			body = fmt.Sprintf(`{"account_id":%q,"test":%v}`, shop, test)
		case r.Method == "POST" && r.URL.Path == "/v3/payments":
			body = fmt.Sprintf(`{"id":"invoice-%s","test":%v,"confirmation":{"confirmation_url":"https://yoomoney.ru/pay"}}`, shop, test)
		case strings.HasPrefix(r.URL.Path, "/v3/payments/invoice-"):
			body = fmt.Sprintf(`{"id":"invoice-%s","status":"succeeded","paid":true,"test":%v,"amount":{"value":"199.00","currency":"RUB"}}`, shop, test)
		default:
			t.Fatalf("unexpected provider request %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	e.s.d.Addons = addons.New(t.TempDir(), "", "0.5.0.10", e.s.d.Log, e.s.d.Now)
	e.s.d.Addons.EnableYooKassa()
	_, err := e.s.SetAddonConfig(ctx, "yookassa", true, addons.Settings{"test_mode": true, "test_shop_id": "111", "test_secret_key": "test-secret", "shop_id": "222", "secret_key": "live-secret"})
	must(t, err)
	testInvoice := e.invoice(555, 0, "addon:yookassa")
	if !strings.HasPrefix(testInvoice.ExternalID.String, "test:") {
		t.Fatal("test invoice not identified")
	}
	_, err = e.s.SetAddonConfig(ctx, "yookassa", true, addons.Settings{"test_mode": false})
	must(t, err)
	liveInvoice := e.invoice(555, 0, "addon:yookassa")
	if liveInvoice.ID == testInvoice.ID || !strings.HasPrefix(liveInvoice.ExternalID.String, "live:") {
		t.Fatal("test invoice reused in live mode")
	}
	// Pending test invoices retain their original shop and still issue once after switching.
	for _, invoice := range []string{testInvoice.ExternalID.String, testInvoice.ExternalID.String, liveInvoice.ExternalID.String} {
		must(t, e.s.checkAddon(ctx, "addon:yookassa", invoice))
	}
	links, err := e.st.Q.ListTgLinksOf(ctx, 555)
	must(t, err)
	if len(links) != 2 || e.tg.told() != 2 {
		t.Fatal("payment did not issue exactly once", len(links), e.tg.told())
	}
	totals, err := e.st.Q.CustomerPurchases(ctx, 555)
	must(t, err)
	if totals.Purchases != 1 || totals.RublesKopecks != 19900 || totals.Days != 30 {
		t.Fatal("test payments included in real purchase totals", totals)
	}
}
