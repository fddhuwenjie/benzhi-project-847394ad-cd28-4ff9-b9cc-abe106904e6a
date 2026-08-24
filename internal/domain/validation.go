package domain

import (
	"strings"
	"time"
)

func ValidateDraft(data DraftData) []Issue {
	var out []Issue
	out = append(out, validateSafetyCollections(data)...)
	if strings.TrimSpace(data.SpaceID) == "" {
		out = append(out, issue("SPACE_REQUIRED", "space_id", "必须登记受限空间标识"))
	}
	if data.PlannedStart.IsZero() {
		out = append(out, issue("START_REQUIRED", "planned_start", "必须填写计划开始时间"))
	}
	if data.PlannedEnd.IsZero() {
		out = append(out, issue("END_REQUIRED", "planned_end", "必须填写计划结束时间"))
	}
	if !data.PlannedStart.IsZero() && !data.PlannedEnd.After(data.PlannedStart) {
		out = append(out, issue("INVALID_WINDOW", "planned_end", "计划结束时间必须晚于开始时间"))
	}
	out = append(out, validatePeople(data.Workers, data.Attendant)...)
	if len(data.GasReadings) == 0 {
		out = append(out, issue("READINGS_REQUIRED", "gas_readings", "至少需要一条气体检测读数"))
	}
	for i, r := range data.GasReadings {
		field := "gas_readings"
		if strings.TrimSpace(r.Gas) == "" || strings.TrimSpace(r.Unit) == "" || r.MeasuredAt.IsZero() {
			out = append(out, issue("READING_INCOMPLETE", field, "气体名称、单位和检测时间不能为空"))
		}
		if strings.EqualFold(r.Gas, "O2") && (r.Value < 19.5 || r.Value > 23.5) {
			out = append(out, issue("OXYGEN_UNSAFE", field, "氧气体积分数必须在 19.5% 至 23.5% 之间"))
		}
		if strings.EqualFold(r.Gas, "LEL") && r.Value >= 10 {
			out = append(out, issue("LEL_UNSAFE", field, "可燃气体不得达到爆炸下限的 10%"))
		}
		if r.MaximumSafe != nil && r.Value > *r.MaximumSafe {
			out = append(out, issue("GAS_LIMIT_EXCEEDED", field, "第 "+itoa(i+1)+" 条检测读数超过安全限值"))
		}
	}
	if len(data.IsolationPoints) == 0 {
		out = append(out, issue("ISOLATION_REQUIRED", "isolation_points", "至少需要一个隔离点"))
	}
	for _, v := range data.IsolationPoints {
		if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.Description) == "" {
			out = append(out, issue("ISOLATION_INCOMPLETE", "isolation_points", "隔离点标识和描述不能为空"))
		}
	}
	if len(data.ControlMeasures) == 0 {
		out = append(out, issue("CONTROL_REQUIRED", "control_measures", "至少需要一项风险控制措施"))
	}
	for _, v := range data.ControlMeasures {
		if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.Description) == "" {
			out = append(out, issue("CONTROL_INCOMPLETE", "control_measures", "控制措施标识和描述不能为空"))
		}
	}
	return out
}

// ValidateDraftShape only rejects malformed or unbounded draft collections.
// Completeness and dynamic safety are deliberately evaluated by preflight and submit.
func ValidateDraftShape(data DraftData) []Issue {
	return validateSafetyCollections(data)
}

type PreflightReport struct {
	PermitID           string    `json:"permit_id"`
	Revision           int64     `json:"revision"`
	EvaluatedAt        time.Time `json:"evaluated_at"`
	ReadyForSubmission bool      `json:"ready_for_submission"`
	Issues             []Issue   `json:"issues"`
}

func Preflight(p *WorkPermit, now time.Time) PreflightReport {
	issues := ValidateForSubmission(p, now)
	if issues == nil {
		issues = []Issue{}
	}
	return PreflightReport{
		PermitID: p.ID, Revision: p.Revision, EvaluatedAt: now.UTC(),
		ReadyForSubmission: len(issues) == 0, Issues: issues,
	}
}

func ValidateForSubmission(p *WorkPermit, now time.Time) []Issue {
	issues := ValidateDraft(DraftDataFromPermit(p))
	if !p.PlannedEnd.After(now) {
		issues = append(issues, issue("PERMIT_EXPIRED", "planned_end", "计划作业时段已经结束"))
	}
	for _, r := range p.GasReadings {
		if r.MeasuredAt.After(now.Add(5*time.Minute)) || now.Sub(r.MeasuredAt) > 30*time.Minute {
			issues = append(issues, issue("READING_STALE", "gas_readings", "气体检测须在提交前 30 分钟内完成"))
		}
	}
	for _, v := range p.IsolationPoints {
		if !v.Confirmed {
			issues = append(issues, issue("ISOLATION_UNCONFIRMED", "isolation_points", "所有隔离点都必须确认"))
		}
	}
	for _, v := range p.ControlMeasures {
		if !v.Completed {
			issues = append(issues, issue("CONTROL_INCOMPLETE", "control_measures", "所有风险控制措施都必须落实"))
		}
	}
	return issues
}

func ValidateActivation(p *WorkPermit, now time.Time) []Issue {
	out := ValidateActivationWindow(p, now)
	for _, r := range p.GasReadings {
		if now.Sub(r.MeasuredAt) > 30*time.Minute || r.MeasuredAt.After(now.Add(5*time.Minute)) {
			out = append(out, issue("READING_STALE", "gas_readings", "批准快照中的气体检测已经失效"))
		}
	}
	for _, v := range p.IsolationPoints {
		if !v.Confirmed {
			out = append(out, issue("ISOLATION_UNCONFIRMED", "isolation_points", "批准后的隔离点状态失效"))
		}
	}
	for _, v := range p.ControlMeasures {
		if !v.Completed {
			out = append(out, issue("CONTROL_INCOMPLETE", "control_measures", "批准后的控制措施状态失效"))
		}
	}
	return out
}

func ValidateActivationWindow(p *WorkPermit, now time.Time) []Issue {
	var out []Issue
	if now.Before(p.PlannedStart) {
		out = append(out, issue("TOO_EARLY", "planned_start", "尚未到计划作业开始时间"))
	}
	if !now.Before(p.PlannedEnd) {
		out = append(out, issue("PERMIT_EXPIRED", "planned_end", "许可计划时段已经结束"))
	}
	return out
}

func validatePeople(workers []Worker, attendant Worker) []Issue {
	var out []Issue
	if len(workers) == 0 {
		out = append(out, issue("WORKERS_REQUIRED", "workers", "至少登记一名作业人员"))
	}
	seen := map[string]bool{}
	for _, w := range workers {
		id := strings.TrimSpace(w.ID)
		if id == "" || strings.TrimSpace(w.Name) == "" {
			out = append(out, issue("WORKER_INCOMPLETE", "workers", "作业人员标识和姓名不能为空"))
			continue
		}
		if seen[id] {
			out = append(out, issue("WORKER_DUPLICATE", "workers", "作业人员标识不能重复"))
		}
		seen[id] = true
	}
	if strings.TrimSpace(attendant.ID) == "" || strings.TrimSpace(attendant.Name) == "" {
		out = append(out, issue("ATTENDANT_REQUIRED", "attendant", "必须登记监护人"))
	}
	if seen[strings.TrimSpace(attendant.ID)] {
		out = append(out, issue("ROLE_CONFLICT", "attendant", "监护人不能同时作为进入空间的作业人员"))
	}
	return out
}

func DraftDataFromPermit(p *WorkPermit) DraftData {
	return DraftData{SpaceID: p.SpaceID, PlannedStart: p.PlannedStart, PlannedEnd: p.PlannedEnd, Workers: p.Workers, Attendant: p.Attendant, GasReadings: p.GasReadings, IsolationPoints: p.IsolationPoints, ControlMeasures: p.ControlMeasures}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	b := [20]byte{}
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
