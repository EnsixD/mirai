package node

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"mirai/internal/nodeapi"
)

func TestNativeUpdateRequestAndStatus(t *testing.T) {
	e := &Engine{dataDir: t.TempDir(), version: "0.5.1"}
	h := Handler(e, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/update", nil))
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	t.Setenv("MIRAI_NATIVE_UPDATE", "1")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/update", nil))
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	if _, err := os.Stat(filepath.Join(e.dataDir, "update", "request")); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/update", nil))
	var status nodeapi.UpdateStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Supported || !status.Requested || status.Version != "0.5.1" {
		t.Fatal(status)
	}
}
