package domain

import "time"

type AuditEvent struct {
	ID              string       `json:"id"`
	EventType       string       `json:"event_type"`
	OccurredAt      time.Time    `json:"occurred_at"`
	Sequence        int          `json:"sequence"`
	ActorID         string       `json:"actor_id,omitempty"`
	RequestID       string       `json:"request_id,omitempty"`
	FromStatus      PermitStatus `json:"from_status,omitempty"`
	ToStatus        PermitStatus `json:"to_status,omitempty"`
	ReviewID        string       `json:"review_id,omitempty"`
	ReviewSequence  int          `json:"review_sequence,omitempty"`
	ReviewerID      string       `json:"reviewer_id,omitempty"`
	FindingID       string       `json:"finding_id,omitempty"`
	EvidenceVersion int          `json:"evidence_version,omitempty"`
	Decision        string       `json:"decision,omitempty"`
	Note            string       `json:"note,omitempty"`
	Issues          []Issue      `json:"issues,omitempty"`
}

func NextAction(bundle *PermitBundle) string {
	switch bundle.Permit.Status {
	case StatusDraft:
		return "SUBMIT"
	case StatusPendingReview:
		if review := bundle.LatestReview(); review == nil || review.Decision != DecisionPending {
			return "ASSIGN_REVIEW"
		}
		return "DECIDE_REVIEW"
	case StatusRevisionsRequired:
		if review := bundle.LatestReview(); review != nil && review.AllFindingsAnswered() {
			return "RESUBMIT"
		}
		return "RESPOND_TO_FINDINGS"
	case StatusApproved:
		return "ACTIVATE"
	case StatusActive:
		return "SUBMIT_CLOSURE"
	case StatusClosureReview:
		if len(bundle.ClosureVerifications) > 0 {
			verification := bundle.ClosureVerifications[len(bundle.ClosureVerifications)-1]
			latestVersion := 0
			if len(bundle.Closures) > 0 {
				latestVersion = bundle.Closures[len(bundle.Closures)-1].Version
			} else if bundle.Closure != nil {
				latestVersion = bundle.Closure.Version
				if latestVersion == 0 {
					latestVersion = 1
				}
			}
			if verification.Decision == ClosureDecisionRejected && verification.EvidenceVersion == latestVersion {
				return "RESUBMIT_CLOSURE"
			}
		}
		return "VERIFY_CLOSURE"
	default:
		return "NONE"
	}
}

func TimingFlag(permit *WorkPermit, now time.Time) string {
	if !permit.PlannedEnd.After(now) && permit.Status != StatusClosed {
		return "OVERDUE"
	}
	if permit.Status == StatusApproved && !now.Before(permit.PlannedStart) && now.Before(permit.PlannedEnd) {
		return "ACTIVATION_DUE"
	}
	if now.Before(permit.PlannedStart) && !permit.PlannedStart.After(now.Add(30*time.Minute)) {
		return "UPCOMING"
	}
	return "ON_SCHEDULE"
}
