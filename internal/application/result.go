package application

import (
	"time"

	"confinedpermit/internal/domain"
)

type PermitView struct {
	Permit               *domain.WorkPermit            `json:"permit"`
	Reviews              []domain.ReviewRound          `json:"reviews"`
	Closure              *domain.ClosureEvidence       `json:"closure,omitempty"`
	ClosureVersions      []domain.ClosureEvidence      `json:"closure_versions"`
	ClosureVerifications []domain.ClosureVerification  `json:"closure_verifications"`
	FindingClosure       *domain.FindingClosureSummary `json:"finding_closure,omitempty"`
	Replay               bool                          `json:"replayed,omitempty"`
}

type TimelineView struct {
	PermitID      string                    `json:"permit_id"`
	Status        domain.PermitStatus       `json:"status"`
	CurrentStatus domain.PermitStatus       `json:"current_status"`
	Revision      int64                     `json:"revision"`
	Transitions   []domain.TransitionRecord `json:"transitions"`
	Reviews       []domain.ReviewRound      `json:"reviews"`
	Closure       *domain.ClosureEvidence   `json:"closure,omitempty"`
	TotalCount    int                       `json:"total_count"`
	MatchedCount  int                       `json:"matched_count"`
	Events        []domain.AuditEvent       `json:"events"`
	NextCursor    string                    `json:"next_cursor,omitempty"`
	EvaluatedAt   time.Time                 `json:"evaluated_at"`
}

func viewOf(b *domain.PermitBundle, replay bool) PermitView {
	closures := append([]domain.ClosureEvidence(nil), b.Closures...)
	if len(closures) == 0 && b.Closure != nil {
		closures = append(closures, *b.Closure)
	}
	view := PermitView{Permit: b.Permit, Reviews: b.Reviews, Closure: b.Closure, ClosureVersions: closures, ClosureVerifications: b.ClosureVerifications, Replay: replay}
	if latest := b.LatestReview(); latest != nil && latest.Decision == domain.DecisionRevisionsRequired {
		summary := latest.ClosureSummary()
		view.FindingClosure = &summary
	}
	return view
}
