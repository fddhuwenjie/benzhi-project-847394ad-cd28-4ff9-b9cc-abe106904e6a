package error_chain_http_status

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"confinedpermit/internal/application"
	"confinedpermit/internal/storage/jsonstore"
	"confinedpermit/internal/transport/httpapi"
)

func TestWrappedNotFoundPreservesHTTPStatus(t *testing.T) {
	store, err := jsonstore.Open(filepath.Join(t.TempDir(), "permits.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(application.New(store), slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/permits/permit-missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("不存在许可应返回 404，实际为 %d", resp.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Fatalf("不存在许可应返回 NOT_FOUND，实际为 %q", body.Error.Code)
	}
}
