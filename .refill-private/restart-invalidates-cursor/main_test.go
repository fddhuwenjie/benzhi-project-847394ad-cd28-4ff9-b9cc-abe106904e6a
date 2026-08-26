package restart_invalidates_cursor_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
	"confinedpermit/internal/storage/jsonstore"
	"confinedpermit/internal/transport/httpapi"
)

func TestCursorRemainsUsableAfterServiceRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permits.json")
	store, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	base := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	for i, id := range []string{"permit-old", "permit-new"} {
		permit := domain.NewPermit(id, domain.DraftData{SpaceID: "SPACE-CURSOR", PlannedStart: base, PlannedEnd: base.Add(time.Hour)}, base.Add(time.Duration(i)*time.Minute))
		key := application.IdempotencyKey{Scope: "permits", Action: "create", RequestID: "create-" + id, Digest: "digest-" + id}
		if _, _, err := store.Create(ctx, &domain.PermitBundle{Permit: permit}, key); err != nil {
			t.Fatal(err)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	beforeRestart := httpapi.New(application.New(store), logger)
	first := serve(beforeRestart, "/api/v1/permits?limit=1")
	if first.Code != http.StatusOK {
		t.Fatalf("第一页状态码 = %d", first.Code)
	}
	var page struct {
		Data struct {
			NextCursor string `json:"next_cursor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Data.NextCursor == "" {
		t.Fatal("第一页缺少 next_cursor")
	}

	storeAfterRestart, err := jsonstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	afterRestart := httpapi.New(application.New(storeAfterRestart), logger)
	second := serve(afterRestart, "/api/v1/permits?limit=1&cursor="+url.QueryEscape(page.Data.NextCursor))
	if second.Code != http.StatusOK {
		t.Fatalf("服务重启后 cursor 应继续有效，状态码 = %d，响应 = %s", second.Code, second.Body.String())
	}
}

func serve(handler http.Handler, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}
