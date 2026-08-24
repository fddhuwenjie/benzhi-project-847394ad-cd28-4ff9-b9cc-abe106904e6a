package domain

import (
	"strings"
	"time"
)

type SiteVerificationInput struct {
	GasReadings                []GasReading `json:"gas_readings,omitempty"`
	WorkerIDs                  []string     `json:"worker_ids"`
	AttendantID                string       `json:"attendant_id"`
	ConfirmedIsolationPointIDs []string     `json:"confirmed_isolation_point_ids"`
	ConfirmedControlMeasureIDs []string     `json:"confirmed_control_measure_ids"`
}

type SiteVerification struct {
	GasReadings                []GasReading `json:"gas_readings"`
	WorkerIDs                  []string     `json:"worker_ids"`
	AttendantID                string       `json:"attendant_id"`
	ConfirmedIsolationPointIDs []string     `json:"confirmed_isolation_point_ids"`
	ConfirmedControlMeasureIDs []string     `json:"confirmed_control_measure_ids"`
	VerifiedBy                 string       `json:"verified_by"`
	RequestID                  string       `json:"request_id"`
	VerifiedAt                 time.Time    `json:"verified_at"`
}

func ValidateSiteVerification(snapshot *ApprovedSnapshot, input *SiteVerificationInput, actor, requestID string, now time.Time) (*SiteVerification, []Issue) {
	if snapshot == nil {
		return nil, []Issue{issue("APPROVAL_SNAPSHOT_MISSING", "approved_snapshot", "缺少批准快照")}
	}
	resolved := SiteVerificationInput{}
	if input == nil {
		resolved.GasReadings = cloneReadings(snapshot.GasReadings)
		for _, worker := range snapshot.Workers {
			resolved.WorkerIDs = append(resolved.WorkerIDs, worker.ID)
		}
		resolved.AttendantID = snapshot.Attendant.ID
		for _, point := range snapshot.IsolationPoints {
			if point.Confirmed {
				resolved.ConfirmedIsolationPointIDs = append(resolved.ConfirmedIsolationPointIDs, point.ID)
			}
		}
		for _, control := range snapshot.ControlMeasures {
			if control.Completed {
				resolved.ConfirmedControlMeasureIDs = append(resolved.ConfirmedControlMeasureIDs, control.ID)
			}
		}
	} else {
		resolved = *input
		resolved.GasReadings = cloneReadings(input.GasReadings)
		if resolved.GasReadings == nil {
			resolved.GasReadings = cloneReadings(snapshot.GasReadings)
		}
	}

	var issues []Issue
	approvedWorkers := map[string]bool{}
	for _, worker := range snapshot.Workers {
		approvedWorkers[worker.ID] = true
	}
	seenWorkers := map[string]bool{}
	if len(resolved.WorkerIDs) == 0 {
		issues = append(issues, issue("SITE_WORKERS_REQUIRED", "worker_ids", "现场复核必须登记实际进场人员"))
	}
	for _, raw := range resolved.WorkerIDs {
		id := strings.TrimSpace(raw)
		if !approvedWorkers[id] {
			issues = append(issues, issue("WORKER_OUTSIDE_APPROVED_SCOPE", "worker_ids", "实际进场人员不在批准快照中"))
		}
		if seenWorkers[id] {
			issues = append(issues, issue("SITE_WORKER_DUPLICATE", "worker_ids", "实际进场人员标识不能重复"))
		}
		seenWorkers[id] = true
	}
	if strings.TrimSpace(resolved.AttendantID) == "" {
		issues = append(issues, issue("SITE_ATTENDANT_REQUIRED", "attendant_id", "现场复核必须登记监护人"))
	} else if resolved.AttendantID != snapshot.Attendant.ID {
		issues = append(issues, issue("ATTENDANT_OUTSIDE_APPROVED_SCOPE", "attendant_id", "现场监护人必须与批准快照一致"))
	}
	if seenWorkers[resolved.AttendantID] {
		issues = append(issues, issue("ROLE_CONFLICT", "attendant_id", "监护人不能同时作为实际进场人员"))
	}

	issues = append(issues, validateConfirmationSet(snapshot.IsolationPoints, resolved.ConfirmedIsolationPointIDs, "ISOLATION", "confirmed_isolation_point_ids")...)
	issues = append(issues, validateControlConfirmationSet(snapshot.ControlMeasures, resolved.ConfirmedControlMeasureIDs)...)
	issues = append(issues, validateReadingSet(resolved.GasReadings)...)
	issues = append(issues, validateReadingsAgainstApproval(snapshot.GasReadings, resolved.GasReadings)...)
	if len(resolved.GasReadings) == 0 {
		issues = append(issues, issue("READINGS_REQUIRED", "gas_readings", "现场复核必须具备有效气体检测批次"))
	}
	for _, reading := range resolved.GasReadings {
		if strings.TrimSpace(reading.Gas) == "" || strings.TrimSpace(reading.Unit) == "" || reading.MeasuredAt.IsZero() {
			issues = append(issues, issue("READING_INCOMPLETE", "gas_readings", "气体名称、单位和检测时间不能为空"))
		}
		if strings.EqualFold(reading.Gas, "O2") && (reading.Value < 19.5 || reading.Value > 23.5) {
			issues = append(issues, issue("OXYGEN_UNSAFE", "gas_readings", "氧气体积分数必须在 19.5% 至 23.5% 之间"))
		}
		if strings.EqualFold(reading.Gas, "LEL") && reading.Value >= 10 {
			issues = append(issues, issue("LEL_UNSAFE", "gas_readings", "可燃气体不得达到爆炸下限的 10%"))
		}
		if reading.MaximumSafe != nil && reading.Value > *reading.MaximumSafe {
			issues = append(issues, issue("GAS_LIMIT_EXCEEDED", "gas_readings", "现场检测读数超过安全限值"))
		}
		if reading.MeasuredAt.After(now.Add(5*time.Minute)) || now.Sub(reading.MeasuredAt) > 30*time.Minute {
			issues = append(issues, issue("READING_STALE", "gas_readings", "现场气体检测须在激活前 30 分钟内完成"))
		}
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return &SiteVerification{
		GasReadings: resolved.GasReadings, WorkerIDs: append([]string(nil), resolved.WorkerIDs...), AttendantID: resolved.AttendantID,
		ConfirmedIsolationPointIDs: append([]string(nil), resolved.ConfirmedIsolationPointIDs...),
		ConfirmedControlMeasureIDs: append([]string(nil), resolved.ConfirmedControlMeasureIDs...),
		VerifiedBy:                 actor, RequestID: requestID, VerifiedAt: now.UTC(),
	}, nil
}

func validateReadingsAgainstApproval(approved, actual []GasReading) []Issue {
	want := map[string]GasReading{}
	for _, reading := range approved {
		want[strings.ToUpper(strings.TrimSpace(reading.Gas))] = reading
	}
	seen := map[string]bool{}
	var issues []Issue
	for _, reading := range actual {
		gas := strings.ToUpper(strings.TrimSpace(reading.Gas))
		expected, ok := want[gas]
		if !ok {
			issues = append(issues, issue("GAS_OUTSIDE_APPROVED_SCOPE", "gas_readings", "现场检测包含批准快照之外的气体种类"))
			continue
		}
		seen[gas] = true
		if strings.TrimSpace(reading.Unit) != strings.TrimSpace(expected.Unit) {
			issues = append(issues, issue("GAS_UNIT_MISMATCH", "gas_readings", "现场检测单位与批准快照不一致"))
		}
		if !sameLimit(reading.MaximumSafe, expected.MaximumSafe) {
			issues = append(issues, issue("GAS_THRESHOLD_MISMATCH", "gas_readings", "现场检测安全阈值与批准快照不一致"))
		}
	}
	for gas := range want {
		if !seen[gas] {
			issues = append(issues, issue("APPROVED_GAS_MISSING", "gas_readings", "现场检测缺少批准快照中的气体种类"))
		}
	}
	return issues
}

func sameLimit(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateConfirmationSet(approved []IsolationPoint, confirmed []string, prefix, field string) []Issue {
	want := map[string]bool{}
	for _, item := range approved {
		want[item.ID] = true
	}
	return validateIDs(want, confirmed, prefix, field)
}

func validateControlConfirmationSet(approved []ControlMeasure, confirmed []string) []Issue {
	want := map[string]bool{}
	for _, item := range approved {
		want[item.ID] = true
	}
	return validateIDs(want, confirmed, "CONTROL", "confirmed_control_measure_ids")
}

func validateIDs(want map[string]bool, got []string, prefix, field string) []Issue {
	seen := map[string]bool{}
	var issues []Issue
	for _, raw := range got {
		id := strings.TrimSpace(raw)
		if !want[id] {
			issues = append(issues, issue(prefix+"_OUTSIDE_APPROVED_SCOPE", field, "现场确认项不在批准快照中"))
		}
		if seen[id] {
			issues = append(issues, issue(prefix+"_CONFIRMATION_DUPLICATE", field, "现场确认标识不能重复"))
		}
		seen[id] = true
	}
	for id := range want {
		if !seen[id] {
			issues = append(issues, issue(prefix+"_CONFIRMATION_MISSING", field, "批准快照中的项目尚未现场确认"))
		}
	}
	return issues
}
