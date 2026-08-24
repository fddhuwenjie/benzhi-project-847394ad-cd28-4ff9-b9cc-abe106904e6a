package domain

import "time"

type ApprovedSnapshot struct {
	ApprovedBy      string           `json:"approved_by"`
	ApprovedAt      time.Time        `json:"approved_at"`
	SourceRevision  int64            `json:"source_revision"`
	Workers         []Worker         `json:"workers"`
	Attendant       Worker           `json:"attendant"`
	GasReadings     []GasReading     `json:"gas_readings"`
	IsolationPoints []IsolationPoint `json:"isolation_points"`
	ControlMeasures []ControlMeasure `json:"control_measures"`
}

func FreezeApproval(p *WorkPermit, reviewer string, now time.Time) *ApprovedSnapshot {
	return &ApprovedSnapshot{
		ApprovedBy: reviewer, ApprovedAt: now.UTC(), SourceRevision: p.Revision,
		Workers: cloneWorkers(p.Workers), Attendant: p.Attendant,
		GasReadings: cloneReadings(p.GasReadings), IsolationPoints: cloneIsolations(p.IsolationPoints),
		ControlMeasures: cloneControls(p.ControlMeasures),
	}
}
