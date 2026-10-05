package addons

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// YooKassa runs directly in the panel; no marketplace container is required.
func YooKassaInfo() Info {
	return Info{ID: "yookassa", Protocol: Protocol, Version: "native", Name: Text{"ru": "ЮKassa", "en": "YooKassa"}, Currencies: []string{"RUB"}, Capabilities: []string{"refund"}, Help: Text{
		"ru": "Сохраните реквизиты боевого и тестового магазинов. Тестовый режим использует отдельный магазин ЮKassa: оплаты выдаются автоматически, но не входят в сумму реальных покупок. В обоих кабинетах настройте HTTP-уведомления payment.succeeded и payment.canceled по адресу ниже.",
		"en": "Save separate live and test shop credentials. Test payments issue subscriptions automatically but do not count as real purchases. Configure payment.succeeded and payment.canceled notifications in both shops using the URL below."}, Settings: []Field{
		{Key: "test_mode", Type: "bool", Label: Text{"ru": "Тестовый режим", "en": "Test mode"}},
		{Key: "shop_id", Type: "string", Label: Text{"ru": "Боевой магазин · Shop ID", "en": "Live shop · Shop ID"}, Pattern: `^[0-9]+$`},
		{Key: "secret_key", Type: "string", Secret: true, Label: Text{"ru": "Боевой магазин · секретный ключ", "en": "Live shop · secret key"}},
		{Key: "test_shop_id", Type: "string", Label: Text{"ru": "Тестовый магазин · Shop ID", "en": "Test shop · Shop ID"}, Pattern: `^[0-9]+$`},
		{Key: "test_secret_key", Type: "string", Secret: true, Label: Text{"ru": "Тестовый магазин · секретный ключ", "en": "Test shop · secret key"}},
	}}
}

type yooPayment struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Paid   bool   `json:"paid"`
	Test   bool   `json:"test"`
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Confirmation struct {
		URL string `json:"confirmation_url"`
	} `json:"confirmation"`
}

func YooKassaConfigured(s Settings) bool {
	test, _ := s["test_mode"].(bool)
	_, _, err := yooCredentials(s, test)
	return err == nil
}

func yooCredentials(s Settings, test bool) (string, string, error) {
	prefix := ""
	if test {
		prefix = "test_"
	}
	shop, _ := s[prefix+"shop_id"].(string)
	secret, _ := s[prefix+"secret_key"].(string)
	if shop == "" || secret == "" {
		return "", "", &Error{Status: 422, Code: "bad_settings", Message: "Fill the selected shop ID and secret key"}
	}
	return shop, secret, nil
}

func (c *Client) yooCall(ctx context.Context, method, path string, s Settings, test bool, key string, body, out any) error {
	shop, secret, err := yooCredentials(s, test)
	if err != nil {
		return err
	}
	// Reuse the bounded adapter transport and response validation, with Basic auth.
	clone := *c
	clone.token = ""
	clone.yoo = false
	clone.authUser, clone.authSecret = shop, secret
	clone.idempotency = key
	return clone.call(ctx, method, path, body, out)
}

func (c *Client) yooCheck(ctx context.Context, s Settings) error {
	test, _ := s["test_mode"].(bool)
	var me struct {
		Test      bool   `json:"test"`
		AccountID string `json:"account_id"`
	}
	if err := c.yooCall(ctx, http.MethodGet, "/me", s, test, "", nil, &me); err != nil {
		return err
	}
	shop, _, _ := yooCredentials(s, test)
	if me.Test != test || me.AccountID != shop {
		return &Error{Status: 422, Code: "bad_credentials", Message: "Shop credentials do not match the selected test/live mode"}
	}
	return nil
}

func yooID(id string, currentTest bool) (string, bool) {
	if value, ok := strings.CutPrefix(id, "test:"); ok {
		return value, true
	}
	if value, ok := strings.CutPrefix(id, "live:"); ok {
		return value, false
	}
	return id, currentTest // Preserve invoices created by older marketplace adapters.
}

func (c *Client) yooInvoice(ctx context.Context, r InvoiceRequest) (Invoice, error) {
	test, _ := r.Settings["test_mode"].(bool)
	if r.Amount <= 0 || r.Currency != "RUB" {
		return Invoice{}, &Error{Status: 422, Code: "bad_request"}
	}
	var p yooPayment
	body := map[string]any{"amount": map[string]string{"value": fmt.Sprintf("%d.%02d", r.Amount/100, r.Amount%100), "currency": "RUB"}, "capture": true, "confirmation": map[string]string{"type": "redirect", "return_url": r.ReturnURL}, "description": r.Description, "metadata": map[string]string{"mirai_payment_id": strconv.FormatInt(r.PaymentID, 10)}}
	if err := c.yooCall(ctx, http.MethodPost, "/payments", r.Settings, test, r.IdempotencyKey, body, &p); err != nil {
		return Invoice{}, err
	}
	if p.ID == "" || p.Test != test || !strings.HasPrefix(p.Confirmation.URL, "https://") {
		return Invoice{}, &Error{Code: "adapter_bad_invoice"}
	}
	prefix := "live:"
	if test {
		prefix = "test:"
	}
	return Invoice{ExternalID: prefix + p.ID, PayURL: p.Confirmation.URL}, nil
}

func (c *Client) yooStatus(ctx context.Context, s Settings, id string) (Status, error) {
	current, _ := s["test_mode"].(bool)
	id, test := yooID(id, current)
	var p yooPayment
	if err := c.yooCall(ctx, http.MethodGet, "/payments/"+url.PathEscape(id), s, test, "", nil, &p); err != nil {
		return Status{}, err
	}
	if p.ID != id || p.Test != test {
		return Status{}, &Error{Status: 422, Code: "bad_payment"}
	}
	parts := strings.Split(p.Amount.Value, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return Status{}, &Error{Code: "bad_amount"}
	}
	rub, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || rub < 0 || rub > 1000000000 {
		return Status{}, &Error{Code: "bad_amount"}
	}
	cents, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || cents < 0 || cents > 99 {
		return Status{}, &Error{Code: "bad_amount"}
	}
	status := "pending"
	if p.Status == "succeeded" && p.Paid {
		status = "paid"
	}
	if p.Status == "canceled" {
		status = "canceled"
	}
	return Status{Status: status, Amount: rub*100 + cents, Currency: p.Amount.Currency}, nil
}

func yooWebhook(body []byte) (string, error) {
	var notification struct {
		Type   string     `json:"type"`
		Event  string     `json:"event"`
		Object yooPayment `json:"object"`
	}
	if json.Unmarshal(body, &notification) != nil {
		return "", &Error{Status: 400, Code: "bad_request"}
	}
	if notification.Type != "notification" || (notification.Event != "payment.succeeded" && notification.Event != "payment.canceled") {
		return "", nil
	}
	if notification.Object.ID == "" {
		return "", &Error{Status: 400, Code: "bad_request"}
	}
	prefix := "live:"
	if notification.Object.Test {
		prefix = "test:"
	}
	// Notifications are hints only; billing checks the payment directly using shop credentials.
	return prefix + notification.Object.ID, nil
}

func (c *Client) yooRefund(ctx context.Context, s Settings, id string, amount int64, key string) error {
	current, _ := s["test_mode"].(bool)
	id, test := yooID(id, current)
	return c.yooCall(ctx, http.MethodPost, "/refunds", s, test, key, map[string]any{"payment_id": id, "amount": map[string]string{"value": fmt.Sprintf("%d.%02d", amount/100, amount%100), "currency": "RUB"}}, nil)
}
