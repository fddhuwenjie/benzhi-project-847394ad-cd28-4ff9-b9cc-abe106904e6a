package domain

import (
	"strings"
	"time"
)

const (
	maximumPermitDuration = 24 * time.Hour
	maximumWorkers        = 50
	maximumReadings       = 30
	maximumSafetyItems    = 100
	maximumReadingSpread  = 10 * time.Minute
)

func validateSafetyCollections(data DraftData) []Issue {
	var issues []Issue
	if !data.PlannedStart.IsZero() && !data.PlannedEnd.IsZero() && data.PlannedEnd.Sub(data.PlannedStart) > maximumPermitDuration {
		issues = append(issues, issue("WINDOW_TOO_LONG", "planned_end", "单张许可的计划作业时段不能超过 24 小时"))
	}
	if len(data.Workers) > maximumWorkers {
		issues = append(issues, issue("TOO_MANY_WORKERS", "workers", "单张许可最多登记 50 名作业人员"))
	}
	if len(data.GasReadings) > maximumReadings {
		issues = append(issues, issue("TOO_MANY_READINGS", "gas_readings", "单张许可最多登记 30 条气体检测读数"))
	}
	if len(data.IsolationPoints) > maximumSafetyItems {
		issues = append(issues, issue("TOO_MANY_ISOLATIONS", "isolation_points", "单张许可最多登记 100 个隔离点"))
	}
	if len(data.ControlMeasures) > maximumSafetyItems {
		issues = append(issues, issue("TOO_MANY_CONTROLS", "control_measures", "单张许可最多登记 100 项控制措施"))
	}
	issues = append(issues, validateReadingSet(data.GasReadings)...)
	issues = append(issues, validateIsolationSet(data.IsolationPoints)...)
	issues = append(issues, validateControlSet(data.ControlMeasures)...)
	return issues
}

func validateReadingSet(readings []GasReading) []Issue {
	var issues []Issue
	seen := map[string]bool{}
	hasOxygen, hasLEL := false, false
	var earliest, latest time.Time
	for _, reading := range readings {
		gas := strings.ToUpper(strings.TrimSpace(reading.Gas))
		if gas != "" {
			if seen[gas] {
				issues = append(issues, issue("READING_DUPLICATE", "gas_readings", "同一气体在检测批次中只能登记一次"))
			}
			seen[gas] = true
		}
		switch gas {
		case "O2", "OXYGEN":
			hasOxygen = true
		case "LEL":
			hasLEL = true
		}
		if reading.Value < 0 {
			issues = append(issues, issue("READING_NEGATIVE", "gas_readings", "气体检测读数不能为负数"))
		}
		if reading.MaximumSafe != nil && *reading.MaximumSafe <= 0 {
			issues = append(issues, issue("GAS_LIMIT_INVALID", "gas_readings", "自定义气体安全上限必须大于零"))
		}
		if reading.MeasuredAt.IsZero() {
			continue
		}
		if earliest.IsZero() || reading.MeasuredAt.Before(earliest) {
			earliest = reading.MeasuredAt
		}
		if latest.IsZero() || reading.MeasuredAt.After(latest) {
			latest = reading.MeasuredAt
		}
	}
	if len(readings) > 0 && !hasOxygen {
		issues = append(issues, issue("OXYGEN_READING_REQUIRED", "gas_readings", "检测批次必须包含氧气读数"))
	}
	if len(readings) > 0 && !hasLEL {
		issues = append(issues, issue("LEL_READING_REQUIRED", "gas_readings", "检测批次必须包含可燃气体爆炸下限读数"))
	}
	if !earliest.IsZero() && latest.Sub(earliest) > maximumReadingSpread {
		issues = append(issues, issue("READING_BATCH_SCATTERED", "gas_readings", "同一检测批次的读数时间跨度不能超过 10 分钟"))
	}
	return issues
}

func validateIsolationSet(points []IsolationPoint) []Issue {
	seen := map[string]bool{}
	var issues []Issue
	for _, point := range points {
		id := strings.TrimSpace(point.ID)
		if id == "" {
			continue
		}
		if seen[id] {
			issues = append(issues, issue("ISOLATION_DUPLICATE", "isolation_points", "隔离点标识不能重复"))
		}
		seen[id] = true
	}
	return issues
}

func validateControlSet(controls []ControlMeasure) []Issue {
	seen := map[string]bool{}
	var issues []Issue
	for _, control := range controls {
		id := strings.TrimSpace(control.ID)
		if id == "" {
			continue
		}
		if seen[id] {
			issues = append(issues, issue("CONTROL_DUPLICATE", "control_measures", "控制措施标识不能重复"))
		}
		seen[id] = true
	}
	return issues
}
