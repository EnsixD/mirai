package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mirai/internal/fsutil"
	"mirai/internal/nodeapi"
)

func (e *Engine) updateStatus() nodeapi.UpdateStatus {
	out := nodeapi.UpdateStatus{Version: e.version, Supported: os.Getenv("MIRAI_NATIVE_UPDATE") == "1"}
	raw, _ := os.ReadFile(filepath.Join(e.dataDir, "update", "status.json"))
	_ = json.Unmarshal(raw, &out)
	_, err := os.Stat(filepath.Join(e.dataDir, "update", "request"))
	out.Requested = err == nil
	return out
}
func (e *Engine) requestUpdate() error {
	dir := filepath.Join(e.dataDir, "update")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"at": time.Now().UTC().Format(time.RFC3339)})
	return fsutil.WriteFileAtomic(filepath.Join(dir, "request"), data, 0600)
}
