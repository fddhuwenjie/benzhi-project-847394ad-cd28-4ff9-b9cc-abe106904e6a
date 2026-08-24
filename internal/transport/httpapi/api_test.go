package httpapi

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
)

type apiResult struct {
	Data struct {
		Permit struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Revision int64  `json:"revision"`
		} `json:"permit"`
	} `json:"data"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func newTestAPI(t *testing.T) *httptest.Server {
	t.Helper()
	store, err := jsonstore.Open(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(New(application.New(store), logger))
	t.Cleanup(server.Close)
	return server
}

func requestJSON(t *testing.T, method, url string, body any) (int, apiResult) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result apiResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, result
}

func TestAPIMapsRevisionConflictAndStrictJSON(t *testing.T) {
	server := newTestAPI(t)
	now := time.Now().UTC()
	draft := map[string]any{
		"space_id": "SPACE-API", "planned_start": now.Add(-time.Minute), "planned_end": now.Add(time.Hour),
		"workers": []map[string]any{{"id": "worker-1", "name": "作业员"}}, "attendant": map[string]any{"id": "attendant-1", "name": "监护员"},
		"gas_readings": []map[string]any{
			{"gas": "O2", "value": 20.9, "unit": "%", "measured_at": now},
			{"gas": "LEL", "value": 0.2, "unit": "%LEL", "measured_at": now},
		},
		"isolation_points": []map[string]any{{"id": "iso-1", "description": "上锁", "confirmed": true}},
		"control_measures": []map[string]any{{"id": "ctl-1", "description": "通风", "completed": true}},
	}
	status, created := requestJSON(t, http.MethodPost, server.URL+"/api/v1/permits", map[string]any{"actor_id": "owner-1", "request_id": "api-create", "draft": draft})
	if status != http.StatusCreated {
		t.Fatalf("创建状态码 = %d, error=%s", status, created.Error.Code)
	}
	id := created.Data.Permit.ID
	status, submitted := requestJSON(t, http.MethodPost, server.URL+"/api/v1/permits/"+id+"/submit", map[string]any{"actor_id": "owner-1", "request_id": "api-submit", "expected_revision": int64(1)})
	if status != http.StatusOK || submitted.Data.Permit.Status != "PENDING_REVIEW" {
		t.Fatalf("提交失败: %d %#v", status, submitted)
	}
	status, conflict := requestJSON(t, http.MethodPost, server.URL+"/api/v1/permits/"+id+"/reviews/assign", map[string]any{"actor_id": "dispatcher-1", "request_id": "api-assign", "expected_revision": int64(1), "reviewer_id": "reviewer-1"})
	if status != http.StatusConflict || conflict.Error.Code != "REVISION_CONFLICT" {
		t.Fatalf("revision 冲突映射错误: %d %#v", status, conflict)
	}
	status, invalid := requestJSON(t, http.MethodPost, server.URL+"/api/v1/permits/"+id+"/reviews/assign", map[string]any{"actor_id": "dispatcher-1", "request_id": "api-invalid", "expected_revision": int64(2), "reviewer_id": "reviewer-1", "unexpected": true})
	if status != http.StatusBadRequest || invalid.Error.Code != "INVALID_JSON" {
		t.Fatalf("未知字段映射错误: %d %#v", status, invalid)
	}
}

func TestPreflightIsReadOnlyForIncompleteDraft(t *testing.T) {
	server := newTestAPI(t)
	now := time.Now().UTC()
	draft := map[string]any{
		"space_id": "SPACE-PREFLIGHT", "planned_start": now.Add(-time.Minute), "planned_end": now.Add(time.Hour),
		"workers": []map[string]any{{"id": "worker-1", "name": "作业员"}}, "attendant": map[string]any{},
		"gas_readings": []map[string]any{
			{"gas": "O2", "value": 20.9, "unit": "%", "measured_at": now.Add(-45 * time.Minute)},
			{"gas": "LEL", "value": 0.2, "unit": "%LEL", "measured_at": now.Add(-45 * time.Minute)},
		},
		"isolation_points": []map[string]any{{"id": "iso-1", "description": "上锁", "confirmed": true}},
		"control_measures": []map[string]any{{"id": "ctl-1", "description": "通风", "completed": true}},
	}
	status, created := requestJSON(t, http.MethodPost, server.URL+"/api/v1/permits", map[string]any{"actor_id": "owner-1", "request_id": "preflight-create", "draft": draft})
	if status != http.StatusCreated {
		t.Fatalf("创建不完整草稿失败: %d %#v", status, created)
	}
	id := created.Data.Permit.ID
	resp, err := http.Get(server.URL + "/api/v1/permits/" + id + "/preflight")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Data struct {
			Revision           int64                             `json:"revision"`
			ReadyForSubmission bool                              `json:"ready_for_submission"`
			Issues             []struct{ Code, Category string } `json:"issues"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || result.Data.ReadyForSubmission || result.Data.Revision != 1 {
		t.Fatalf("预检结果错误: status=%d result=%#v", resp.StatusCode, result)
	}
	want := map[string]bool{"ATTENDANT_REQUIRED": false, "READING_STALE": false}
	for _, issue := range result.Data.Issues {
		if _, ok := want[issue.Code]; ok {
			want[issue.Code] = issue.Category != ""
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("预检缺少分类问题 %s: %#v", code, result.Data.Issues)
		}
	}
	resp, err = http.Get(server.URL + "/api/v1/permits/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var current apiResult
	if err := json.NewDecoder(resp.Body).Decode(&current); err != nil {
		t.Fatal(err)
	}
	if current.Data.Permit.Status != "DRAFT" || current.Data.Permit.Revision != 1 {
		t.Fatalf("预检改变了许可: %#v", current.Data.Permit)
	}
}

func TestClosureCanBeRejectedResubmittedAndApproved(t *testing.T) {
	server := newTestAPI(t)
	now := time.Now().UTC()
	draft := map[string]any{
		"space_id": "SPACE-CLOSURE", "planned_start": now.Add(-time.Minute), "planned_end": now.Add(time.Hour),
		"workers": []map[string]any{{"id": "worker-1", "name": "作业员"}}, "attendant": map[string]any{"id": "attendant-1", "name": "监护员"},
		"gas_readings":     []map[string]any{{"gas": "O2", "value": 20.9, "unit": "%", "measured_at": now}, {"gas": "LEL", "value": 0.2, "unit": "%LEL", "measured_at": now}},
		"isolation_points": []map[string]any{{"id": "iso-1", "description": "上锁", "confirmed": true}},
		"control_measures": []map[string]any{{"id": "ctl-1", "description": "通风", "completed": true}},
	}
	status, result := requestJSON(t, http.MethodPost, server.URL+"/api/v1/permits", map[string]any{"actor_id": "owner-1", "request_id": "closure-create", "draft": draft})
	if status != http.StatusCreated {
		t.Fatalf("创建失败: %d %#v", status, result)
	}
	id := result.Data.Permit.ID
	path := server.URL + "/api/v1/permits/" + id
	steps := []struct {
		path string
		body map[string]any
	}{
		{path + "/submit", map[string]any{"actor_id": "owner-1", "request_id": "closure-submit", "expected_revision": 1}},
		{path + "/reviews/assign", map[string]any{"actor_id": "dispatcher-1", "request_id": "closure-assign", "expected_revision": 2, "reviewer_id": "reviewer-1"}},
		{path + "/reviews/decision", map[string]any{"actor_id": "reviewer-1", "request_id": "closure-approve", "expected_revision": 3, "decision": "APPROVED"}},
		{path + "/activate", map[string]any{"actor_id": "owner-1", "request_id": "closure-activate", "expected_revision": 4}},
		{path + "/closure", map[string]any{"actor_id": "owner-1", "request_id": "closure-evidence-1", "expected_revision": 5, "personnel_cleared": true, "tools_accounted": true, "isolations_restored": true, "photo_refs": []string{"photo://v1"}}},
	}
	for _, step := range steps {
		status, result = requestJSON(t, http.MethodPost, step.path, step.body)
		if status != http.StatusOK {
			t.Fatalf("闭环步骤失败 %s: %d %#v", step.path, status, result)
		}
	}
	status, result = requestJSON(t, http.MethodPost, path+"/closure/verify", map[string]any{"actor_id": "verifier-1", "request_id": "closure-reject", "expected_revision": 6, "evidence_version": 1, "decision": "REJECTED", "issues": []map[string]any{{"code": "PHOTO_UNCLEAR", "field": "photo_refs", "message": "照片无法辨识"}}})
	if status != http.StatusOK || result.Data.Permit.Status != "CLOSURE_REVIEW" || result.Data.Permit.Revision != 7 {
		t.Fatalf("退回失败: %d %#v", status, result)
	}
	status, result = requestJSON(t, http.MethodPost, path+"/closure/verify", map[string]any{"actor_id": "verifier-1", "request_id": "closure-too-early", "expected_revision": 7, "evidence_version": 1, "decision": "APPROVED", "note": "通过"})
	if status != http.StatusConflict || result.Error.Code != "EVIDENCE_RESUBMISSION_REQUIRED" {
		t.Fatalf("未重提时应拒绝关闭: %d %#v", status, result)
	}
	status, result = requestJSON(t, http.MethodPost, path+"/closure", map[string]any{"actor_id": "owner-1", "request_id": "closure-evidence-2", "expected_revision": 7, "personnel_cleared": true, "tools_accounted": true, "isolations_restored": true, "photo_refs": []string{"photo://v2"}})
	if status != http.StatusOK || result.Data.Permit.Revision != 8 {
		t.Fatalf("证据重提失败: %d %#v", status, result)
	}
	status, result = requestJSON(t, http.MethodPost, path+"/closure/verify", map[string]any{"actor_id": "verifier-1", "request_id": "closure-old-version", "expected_revision": 8, "evidence_version": 1, "decision": "APPROVED", "note": "通过"})
	if status != http.StatusConflict || result.Error.Code != "EVIDENCE_VERSION_CONFLICT" {
		t.Fatalf("旧证据版本应冲突: %d %#v", status, result)
	}
	status, result = requestJSON(t, http.MethodPost, path+"/closure/verify", map[string]any{"actor_id": "verifier-1", "request_id": "closure-final", "expected_revision": 8, "evidence_version": 2, "decision": "APPROVED", "note": "新证据完整可信"})
	if status != http.StatusOK || result.Data.Permit.Status != "CLOSED" || result.Data.Permit.Revision != 9 {
		t.Fatalf("最终关闭失败: %d %#v", status, result)
	}
}
