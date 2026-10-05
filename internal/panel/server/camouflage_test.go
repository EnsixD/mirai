package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCamouflagePages(t *testing.T) {
	h := Camouflage("")
	for _, p := range []string{"/", "/robots.txt", "/favicon.ico"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), "404 Not Found") {
			t.Fatalf("bare response at %s", p)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("Our public website"), 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	Camouflage(dir).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Body.String() != "Our public website" {
		t.Fatal("custom site not served")
	}
	w = httptest.NewRecorder()
	Camouflage(dir).ServeHTTP(w, httptest.NewRequest("GET", "/missing", nil))
	if w.Code != 404 || strings.Contains(w.Body.String(), "<a href=") {
		t.Fatal("directory listing exposed")
	}
}

func TestPublicSubscriptionCamouflageFallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{color:teal}"), 0600)
	sub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/validtoken" {
			w.Write([]byte("subscription"))
			return
		}
		NotFound(w)
	})
	s := New(nil, sub)
	s.SetSubscriptionURL(func(*http.Request) string { return "https://subs.example.com" })
	s.SetCamouflage(Camouflage(dir))
	for _, test := range []struct {
		path, body string
		status     int
	}{
		{"/style.css", "body{color:teal}", 200},
		{"/validtoken", "subscription", 200},
		{"/unknown", "Service portal", 404},
	} {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", "https://subs.example.com"+test.path, nil))
		if w.Code != test.status || !strings.Contains(w.Body.String(), test.body) {
			t.Fatalf("%s: %d %s", test.path, w.Code, w.Body.String())
		}
	}
}
