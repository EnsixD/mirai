package server

import (
	"mirai/internal/panel/settings"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicSubscriptionRouting(t *testing.T) {
	hit := ""
	s := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = "admin" }), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = r.URL.Path; w.WriteHeader(200) }))
	s.SetPaths(settings.Paths{Admin: "private", Sub: "secret"})
	s.SetSubscriptionURL(func(*http.Request) string { return "http://subs.example.com" })
	r := httptest.NewRequest("GET", "http://subs.example.com/A7b2X9mQ4", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if hit != "/A7b2X9mQ4" || w.Code != 200 {
		t.Fatal(hit, w.Code)
	}
	hit = ""
	r = httptest.NewRequest("GET", "http://other.example.com/A7b2X9mQ4", nil)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if hit != "" || w.Code != 404 {
		t.Fatal("wrong host routed")
	}
	r = httptest.NewRequest("GET", "http://subs.example.com/private/", nil)
	w = httptest.NewRecorder()
	s.SubOnly().ServeHTTP(w, r)
	if hit == "admin" {
		t.Fatal("subscription host exposes admin")
	}
}
