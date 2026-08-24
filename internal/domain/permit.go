package domain

import "time"

type Worker struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GasReading struct {
	Gas         string    `json:"gas"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	MeasuredAt  time.Time `json:"measured_at"`
	MaximumSafe *float64  `json:"maximum_safe,omitempty"`
}

type IsolationPoint struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Confirmed   bool   `json:"confirmed"`
}

type ControlMeasure struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

type WorkPermit struct {
	ID               string            `json:"id"`
	OwnerID          string            `json:"owner_id,omitempty"`
	SpaceID          string            `json:"space_id"`
	Status           PermitStatus      `json:"status"`
	Revision         int64             `json:"revision"`
	PlannedStart     time.Time         `json:"planned_start"`
	PlannedEnd       time.Time         `json:"planned_end"`
	Workers          []Worker          `json:"workers"`
	Attendant        Worker            `json:"attendant"`
	GasReadings      []GasReading      `json:"gas_readings"`
	IsolationPoints  []IsolationPoint  `json:"isolation_points"`
	ControlMeasures  []ControlMeasure  `json:"control_measures"`
	ApprovedSnapshot *ApprovedSnapshot `json:"approved_snapshot,omitempty"`
	SiteVerification *SiteVerification `json:"site_verification,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type DraftData struct {
	SpaceID         string           `json:"space_id"`
	PlannedStart    time.Time        `json:"planned_start"`
	PlannedEnd      time.Time        `json:"planned_end"`
	Workers         []Worker         `json:"workers"`
	Attendant       Worker           `json:"attendant"`
	GasReadings     []GasReading     `json:"gas_readings"`
	IsolationPoints []IsolationPoint `json:"isolation_points"`
	ControlMeasures []ControlMeasure `json:"control_measures"`
}

func NewPermit(id string, data DraftData, now time.Time) *WorkPermit {
	return &WorkPermit{
		ID: id, SpaceID: data.SpaceID, Status: StatusDraft, Revision: 1,
		PlannedStart: data.PlannedStart, PlannedEnd: data.PlannedEnd,
		Workers: cloneWorkers(data.Workers), Attendant: data.Attendant,
		GasReadings: cloneReadings(data.GasReadings), IsolationPoints: cloneIsolations(data.IsolationPoints),
		ControlMeasures: cloneControls(data.ControlMeasures), CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
}

func (p *WorkPermit) ReplaceDraft(data DraftData, now time.Time) error {
	if p.Status != StatusDraft && p.Status != StatusRevisionsRequired {
		return NewConflict("DRAFT_LOCKED", "当前状态不允许修订许可内容")
	}
	p.SpaceID = data.SpaceID
	p.PlannedStart = data.PlannedStart
	p.PlannedEnd = data.PlannedEnd
	p.Workers = cloneWorkers(data.Workers)
	p.Attendant = data.Attendant
	p.GasReadings = cloneReadings(data.GasReadings)
	p.IsolationPoints = cloneIsolations(data.IsolationPoints)
	p.ControlMeasures = cloneControls(data.ControlMeasures)
	p.UpdatedAt = now.UTC()
	return nil
}

func (p *WorkPermit) Transition(to PermitStatus, now time.Time) error {
	if err := RequireTransition(p.Status, to); err != nil {
		return err
	}
	p.Status = to
	p.UpdatedAt = now.UTC()
	return nil
}

func cloneWorkers(in []Worker) []Worker          { return append([]Worker(nil), in...) }
func cloneReadings(in []GasReading) []GasReading { return append([]GasReading(nil), in...) }
func cloneIsolations(in []IsolationPoint) []IsolationPoint {
	return append([]IsolationPoint(nil), in...)
}
func cloneControls(in []ControlMeasure) []ControlMeasure { return append([]ControlMeasure(nil), in...) }
