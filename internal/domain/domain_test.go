package domain

import (
	"testing"
	"time"
)

func validDraft(now time.Time) DraftData {
	return DraftData{
		SpaceID: "SPACE-01", PlannedStart: now.Add(-time.Minute), PlannedEnd: now.Add(time.Hour),
		Workers: []Worker{{ID: "worker-1", Name: "作业员"}}, Attendant: Worker{ID: "attendant-1", Name: "监护员"},
		GasReadings: []GasReading{
			{Gas: "O2", Value: 20.9, Unit: "%", MeasuredAt: now},
			{Gas: "LEL", Value: 0.2, Unit: "%LEL", MeasuredAt: now},
		},
		IsolationPoints: []IsolationPoint{{ID: "iso-1", Description: "阀门上锁", Confirmed: true}},
		ControlMeasures: []ControlMeasure{{ID: "ctl-1", Description: "持续通风", Completed: true}},
	}
}

func TestSubmissionValidationReportsSafetyIssues(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	draft := validDraft(now)
	draft.GasReadings[0].Value = 18.2
	draft.GasReadings[0].MeasuredAt = now.Add(-45 * time.Minute)
	draft.IsolationPoints[0].Confirmed = false
	draft.ControlMeasures[0].Completed = false
	p := NewPermit("permit-1", draft, now)
	issues := ValidateForSubmission(p, now)
	want := map[string]bool{"OXYGEN_UNSAFE": false, "READING_STALE": false, "ISOLATION_UNCONFIRMED": false, "CONTROL_INCOMPLETE": false}
	for _, got := range issues {
		if _, ok := want[got.Code]; ok {
			want[got.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("缺少结构化问题 %s: %#v", code, issues)
		}
	}
}

func TestPermitOnlyAllowsClosedLoopTransitions(t *testing.T) {
	now := time.Now().UTC()
	p := NewPermit("permit-1", validDraft(now), now)
	if err := p.Transition(StatusActive, now); err == nil {
		t.Fatal("DRAFT 不应直接转换到 ACTIVE")
	}
	for _, status := range []PermitStatus{StatusPendingReview, StatusApproved, StatusActive, StatusClosureReview, StatusClosed} {
		if err := p.Transition(status, now); err != nil {
			t.Fatalf("转换到 %s 失败: %v", status, err)
		}
	}
	if p.Status != StatusClosed {
		t.Fatalf("最终状态 = %s", p.Status)
	}
}

func TestReviewRoundRequiresEveryFindingResponse(t *testing.T) {
	now := time.Now().UTC()
	r, err := NewReviewRound("review-1", "permit-1", "reviewer-1", 1, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	findings := []ReviewFinding{{ID: "f-1", Message: "补充检测"}, {ID: "f-2", Message: "确认隔离"}}
	if err := r.Decide(DecisionRevisionsRequired, findings, "reviewer-1", now); err != nil {
		t.Fatal(err)
	}
	if err := r.Respond([]FindingResponse{{FindingID: "f-1", Response: "已补充"}}, "owner-1", now); err != nil {
		t.Fatal(err)
	}
	if r.AllFindingsAnswered() {
		t.Fatal("只回应一个问题时不应允许重新提交")
	}
	if err := r.Respond([]FindingResponse{{FindingID: "f-1", Response: "已补充"}, {FindingID: "f-2", Response: "已确认"}}, "owner-1", now); err != nil {
		t.Fatal(err)
	}
	if !r.AllFindingsAnswered() {
		t.Fatal("全部问题回应后应允许重新提交")
	}
}

func TestClosureEvidenceMustBeComplete(t *testing.T) {
	now := time.Now().UTC()
	e, issues := NewClosureEvidence("permit-1", true, false, true, nil, "owner-1", now)
	if e != nil || len(issues) != 2 {
		t.Fatalf("不完整证据结果 = %#v, %#v", e, issues)
	}
	e, issues = NewClosureEvidence("permit-1", true, true, true, []string{"photo://exit"}, "owner-1", now)
	if e == nil || len(issues) != 0 {
		t.Fatalf("完整证据被拒绝: %#v", issues)
	}
	if err := e.Verify("verifier-1", "", now); err == nil {
		t.Fatal("空核验说明不应关闭许可")
	}
}

func TestReviewResponsesMergeAndCanBeRevised(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	r, err := NewReviewRound("review-merge", "permit-1", "reviewer-1", 1, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Decide(DecisionRevisionsRequired, []ReviewFinding{{ID: "f-1", Message: "问题一"}, {ID: "f-2", Message: "问题二"}, {ID: "f-3", Message: "问题三"}}, "reviewer-1", now); err != nil {
		t.Fatal(err)
	}
	if err := r.RespondWithRequest([]FindingResponse{{FindingID: "f-1", Response: "首次回应"}, {FindingID: "f-2", Response: "已处理"}}, "owner-1", "response-1", now); err != nil {
		t.Fatal(err)
	}
	if err := r.RespondWithRequest([]FindingResponse{{FindingID: "f-3", Response: "已处理"}}, "owner-2", "response-2", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(r.Responses) != 3 || !r.AllFindingsAnswered() {
		t.Fatalf("增量回应未保留原有内容: %#v", r.Responses)
	}
	if err := r.RespondWithRequest([]FindingResponse{{FindingID: "f-1", Response: "修订回应"}}, "owner-2", "response-3", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(r.Responses) != 3 || r.Responses[0].Response != "修订回应" || r.Responses[0].ActorID != "owner-2" {
		t.Fatalf("单项修订结果错误: %#v", r.Responses)
	}
}

func TestSiteVerificationCanRefreshReadingsWithoutChangingApproval(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	draft := validDraft(now.Add(-45 * time.Minute))
	p := NewPermit("permit-site", draft, now.Add(-45*time.Minute))
	snapshot := FreezeApproval(p, "reviewer-1", now.Add(-40*time.Minute))
	input := &SiteVerificationInput{
		GasReadings: []GasReading{
			{Gas: "O2", Value: 20.9, Unit: "%", MeasuredAt: now},
			{Gas: "LEL", Value: 0.2, Unit: "%LEL", MeasuredAt: now},
		},
		WorkerIDs: []string{"worker-1"}, AttendantID: "attendant-1",
		ConfirmedIsolationPointIDs: []string{"iso-1"}, ConfirmedControlMeasureIDs: []string{"ctl-1"},
	}
	verification, issues := ValidateSiteVerification(snapshot, input, "owner-1", "activate-1", now)
	if len(issues) != 0 || verification == nil {
		t.Fatalf("有效的新检测批次被拒绝: %#v", issues)
	}
	if !snapshot.GasReadings[0].MeasuredAt.Equal(now.Add(-45 * time.Minute)) {
		t.Fatal("现场复核不应改写批准快照")
	}
	input.WorkerIDs = []string{"worker-outside"}
	input.ConfirmedIsolationPointIDs = nil
	_, issues = ValidateSiteVerification(snapshot, input, "owner-1", "activate-2", now)
	want := map[string]bool{"WORKER_OUTSIDE_APPROVED_SCOPE": false, "ISOLATION_CONFIRMATION_MISSING": false}
	for _, item := range issues {
		if _, ok := want[item.Code]; ok {
			want[item.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("现场复核缺少问题 %s: %#v", code, issues)
		}
	}
}

func TestClosureRejectionRequiresStructuredIssues(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if _, err := NewClosureVerification("verify-1", 1, ClosureDecisionRejected, "", nil, "verifier-1", "request-1", now); err == nil {
		t.Fatal("退回决定缺少问题项时应被拒绝")
	}
	verification, err := NewClosureVerification("verify-2", 1, ClosureDecisionRejected, "", []Issue{{Code: "PHOTO_UNCLEAR", Field: "photo_refs", Message: "照片无法辨识"}}, "verifier-1", "request-2", now)
	if err != nil || verification.Decision != ClosureDecisionRejected || verification.Issues[0].Category == "" {
		t.Fatalf("结构化退回决定创建失败: %#v %v", verification, err)
	}
}
