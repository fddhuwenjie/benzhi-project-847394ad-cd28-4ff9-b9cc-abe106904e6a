package application

import (
	"time"

	"confinedpermit/internal/domain"
)

type QueueQuery struct {
	Statuses         []domain.PermitStatus
	SpaceID          string
	ReviewerID       string
	PlannedStartFrom *time.Time
	PlannedEndTo     *time.Time
	Limit            int
	Cursor           string
}

type QueueItem struct {
	PermitID         string              `json:"permit_id"`
	SpaceID          string              `json:"space_id"`
	Status           domain.PermitStatus `json:"status"`
	Revision         int64               `json:"revision"`
	PlannedStart     time.Time           `json:"planned_start"`
	PlannedEnd       time.Time           `json:"planned_end"`
	UpdatedAt        time.Time           `json:"updated_at"`
	LatestReviewerID string              `json:"latest_reviewer_id,omitempty"`
	NextAction       string              `json:"next_action"`
	Timing           string              `json:"timing"`
}

type QueueSummary struct {
	StatusCounts map[domain.PermitStatus]int `json:"status_counts"`
	OverdueCount int                         `json:"overdue_count"`
}

type QueueView struct {
	Items       []QueueItem  `json:"items"`
	NextCursor  string       `json:"next_cursor,omitempty"`
	Summary     QueueSummary `json:"summary"`
	EvaluatedAt time.Time    `json:"evaluated_at"`
}

type TimelineQuery struct {
	ActorID      string
	RequestID    string
	FromStatus   domain.PermitStatus
	ToStatus     domain.PermitStatus
	OccurredFrom *time.Time
	OccurredTo   *time.Time
	Limit        int
	Cursor       string
}

type AuditTimelineView struct {
	PermitID      string              `json:"permit_id"`
	CurrentStatus domain.PermitStatus `json:"current_status"`
	Revision      int64               `json:"revision"`
	TotalCount    int                 `json:"total_count"`
	MatchedCount  int                 `json:"matched_count"`
	Events        []domain.AuditEvent `json:"events"`
	NextCursor    string              `json:"next_cursor,omitempty"`
}
