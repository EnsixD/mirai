package subs

import (
	"encoding/base64"
	"net/http"
	"testing"
)

func TestClientRoutingIsolation(t *testing.T) {
	happ := `{"Name":"Happ Mirai","DirectSites":["geosite:ru"]}`
	incy := `{"Name":"INCY Mirai","ProxySites":["example.com"]}`
	cfg := Config{HappRules: happ, INCYRules: incy}
	for _, tc := range []struct{ ua, app, profile string }{
		{"Happ/3", "happ", happ}, {"INCY/1", "incy", incy},
		{"Clash", "", ""}, {"v2rayNG", "", ""},
	} {
		h := http.Header{}
		ClientRoutingHeader(h, tc.ua, cfg)
		want := ""
		if tc.app != "" {
			want = tc.app + "://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(tc.profile))
		}
		if h.Get("routing") != want {
			t.Errorf("%s: unexpected routing header", tc.ua)
		}
	}
}

func TestValidateClientRouting(t *testing.T) {
	for _, bad := range []string{`null`, `[]`, `{}`, `{"Name":""}`, `{"Name":"Mirai","DirectSites":"site"}`, `{"Name":"Mirai","DirectIp":[12]}`} {
		if ValidateClientRouting(bad) == nil {
			t.Errorf("accepted invalid profile: %s", bad)
		}
	}
	for _, good := range []string{"", `{"Name":"Mirai","DirectSites":["geosite:ru"]}`} {
		if err := ValidateClientRouting(good); err != nil {
			t.Fatal(err)
		}
	}
	h := http.Header{}
	ClientRoutingHeader(h, "Happ/3", Config{HappRules: "broken"})
	if h.Get("routing") != "" {
		t.Fatal("invalid profile leaked to subscription")
	}
}

func TestRoutingDeeplinks(t *testing.T) {
	profile := base64.StdEncoding.EncodeToString([]byte(`{"Name":"RoscomVPN","DirectSites":["geosite:ru"]}`))
	for _, app := range []string{"happ", "incy"} {
		link := app + "://routing/onadd/" + profile
		got, err := ClientRoutingLink(link, app)
		if err != nil || got != link {
			t.Fatalf("link changed: %s %v", got, err)
		}
		if _, err := ClientRoutingLink("other://routing/onadd/"+profile, app); err == nil {
			t.Fatal("wrong app accepted")
		}
		if _, err := ClientRoutingLink(app+"://routing/onadd/not-base64", app); err == nil {
			t.Fatal("invalid base64 accepted")
		}
	}
	h := http.Header{}
	link := "happ://routing/onadd/" + profile
	ClientRoutingHeader(h, "Happ/3", Config{HappRules: link})
	if h.Get("routing") != link {
		t.Fatal("deeplink was encoded twice")
	}
}
