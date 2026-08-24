package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func selfCheck(cfg config, logger *slog.Logger) error {
	tempDir, err := os.MkdirTemp("", "confined-permit-selfcheck-")
	if err != nil {
		return fmt.Errorf("创建自检数据目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	rt, err := buildRuntime(filepath.Join(tempDir, "snapshot.json"), logger)
	if err != nil {
		return fmt.Errorf("初始化自检服务: %w", err)
	}
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", cfg.Addr, err)
	}
	server := newHTTPServer(cfg.Addr, rt.handler)
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	probeErr := executeClosedLoop(ctx, newProbeClient(listener.Addr().String()))
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	serveErr := <-errCh
	if probeErr != nil {
		return fmt.Errorf("HTTP 闭环自检失败: %w", probeErr)
	}
	if shutdownErr != nil {
		return fmt.Errorf("关闭自检服务: %w", shutdownErr)
	}
	if !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("自检服务异常退出: %w", serveErr)
	}
	if err := rt.service.Flush(shutdownCtx); err != nil {
		return fmt.Errorf("刷新自检快照: %w", err)
	}
	logger.Info("完整许可闭环自检通过", "final_status", "CLOSED")
	return nil
}

func executeClosedLoop(ctx context.Context, client *probeClient) error {
	now := time.Now().UTC()
	draft := map[string]any{
		"space_id": "TK-SELF-CHECK", "planned_start": now.Add(-time.Minute), "planned_end": now.Add(time.Hour),
		"workers":   []map[string]any{{"id": "worker-1", "name": "自检作业员"}},
		"attendant": map[string]any{"id": "attendant-1", "name": "自检监护员"},
		"gas_readings": []map[string]any{
			{"gas": "O2", "value": 20.9, "unit": "%", "measured_at": now},
			{"gas": "LEL", "value": 0.2, "unit": "%LEL", "measured_at": now},
		},
		"isolation_points": []map[string]any{{"id": "iso-1", "description": "进料阀上锁", "confirmed": true}},
		"control_measures": []map[string]any{{"id": "ctl-1", "description": "强制通风", "completed": true}},
	}
	created, err := client.write(ctx, "/api/v1/permits", map[string]any{"actor_id": "owner-1", "request_id": "self-create", "draft": draft}, http.StatusCreated)
	if err != nil {
		return err
	}
	id, revision := created.Data.Permit.ID, created.Data.Permit.Revision
	if id == "" || created.Data.Permit.Status != "DRAFT" {
		return fmt.Errorf("创建结果缺少许可标识或状态错误")
	}
	path := "/api/v1/permits/" + id
	result, err := client.write(ctx, path+"/submit", action("owner-1", "self-submit-1", revision), http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	replay, err := client.write(ctx, path+"/submit", action("owner-1", "self-submit-1", 1), http.StatusOK)
	if err != nil {
		return err
	}
	if !replay.Data.Replay {
		return fmt.Errorf("重复提交未返回幂等重放标记")
	}
	result, err = client.write(ctx, path+"/reviews/assign", map[string]any{"actor_id": "dispatcher-1", "request_id": "self-assign-1", "expected_revision": revision, "reviewer_id": "reviewer-1"}, http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/reviews/decision", map[string]any{"actor_id": "reviewer-1", "request_id": "self-reject", "expected_revision": revision, "decision": "REVISIONS_REQUIRED", "reason": "补充救援联络确认", "findings": []map[string]any{{"id": "finding-1", "message": "救援联络确认记录不清晰"}}}, http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/reviews/responses", map[string]any{"actor_id": "owner-1", "request_id": "self-response", "expected_revision": revision, "responses": []map[string]any{{"finding_id": "finding-1", "response": "已完成现场联络测试"}}}, http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/submit", action("owner-1", "self-submit-2", revision), http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/reviews/assign", map[string]any{"actor_id": "dispatcher-1", "request_id": "self-assign-2", "expected_revision": revision, "reviewer_id": "reviewer-1"}, http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/reviews/decision", map[string]any{"actor_id": "reviewer-1", "request_id": "self-approve", "expected_revision": revision, "decision": "APPROVED", "reason": "风险控制确认有效"}, http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/activate", action("owner-1", "self-activate", revision), http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/closure", map[string]any{"actor_id": "owner-1", "request_id": "self-closure", "expected_revision": revision, "personnel_cleared": true, "tools_accounted": true, "isolations_restored": true, "photo_refs": []string{"photo://self-check/exit.jpg"}}, http.StatusOK)
	if err != nil {
		return err
	}
	revision = result.Data.Permit.Revision
	result, err = client.write(ctx, path+"/closure/verify", map[string]any{"actor_id": "verifier-1", "request_id": "self-verify", "expected_revision": revision, "note": "人员、工具和隔离恢复情况均已核验"}, http.StatusOK)
	if err != nil {
		return err
	}
	if result.Data.Permit.Status != "CLOSED" {
		return fmt.Errorf("最终许可状态为 %s", result.Data.Permit.Status)
	}
	timeline, err := client.getTimeline(ctx, path+"/timeline")
	if err != nil {
		return err
	}
	if timeline.Data.Status != "CLOSED" || len(timeline.Data.Reviews) != 2 || len(timeline.Data.Transitions) < 8 {
		return fmt.Errorf("时间线缺少完整闭环记录")
	}
	return nil
}

func action(actor, requestID string, revision int64) map[string]any {
	return map[string]any{"actor_id": actor, "request_id": requestID, "expected_revision": revision}
}
