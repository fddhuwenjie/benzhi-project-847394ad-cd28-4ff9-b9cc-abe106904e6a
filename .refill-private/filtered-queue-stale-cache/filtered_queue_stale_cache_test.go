package filtered_queue_stale_cache_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/storage/jsonstore"
	"confinedpermit/internal/transport/httpapi"
)

func TestFilteredQueueRefreshesAfterCreate(t *testing.T) {
	store, err := jsonstore.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(httpapi.New(application.New(store), logger))
	t.Cleanup(server.Close)

	createPermit(t, server.URL, "SPACE-CACHE-A", "create-cache-a")
	if got := draftQueueCount(t, server.URL); got != 1 {
		t.Fatalf("首次队列条目数 = %d，期望 1", got)
	}

	createPermit(t, server.URL, "SPACE-CACHE-B", "create-cache-b")
	if got := draftQueueCount(t, server.URL); got != 2 {
		t.Fatalf("创建第二张许可后的队列条目数 = %d，期望 2", got)
	}
}

func createPermit(t *testing.T, baseURL, spaceID, requestID string) {
	t.Helper()
	now := time.Now().UTC()
	body := map[string]any{
		"actor_id":   "owner-cache",
		"request_id": requestID,
		"draft": map[string]any{
			"space_id":      spaceID,
			"planned_start": now.Add(-time.Minute),
			"planned_end":   now.Add(time.Hour),
			"workers":       []map[string]any{{"id": "worker-1", "name": "作业员"}},
			"attendant":     map[string]any{"id": "attendant-1", "name": "监护员"},
			"gas_readings": []map[string]any{
				{"gas": "O2", "value": 20.9, "unit": "%", "measured_at": now},
				{"gas": "LEL", "value": 0.2, "unit": "%LEL", "measured_at": now},
			},
			"isolation_points": []map[string]any{{"id": "iso-1", "description": "上锁", "confirmed": true}},
			"control_measures": []map[string]any{{"id": "ctl-1", "description": "通风", "completed": true}},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/permits", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		message, _ := io.ReadAll(response.Body)
		t.Fatalf("创建许可返回 %d: %s", response.StatusCode, message)
	}
}

func draftQueueCount(t *testing.T, baseURL string) int {
	t.Helper()
	response, err := http.Get(baseURL + "/api/v1/permits?status=DRAFT")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("查询队列返回 %d", response.StatusCode)
	}
	return len(result.Data.Items)
}
