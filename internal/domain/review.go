package domain

import (
	"strings"
	"time"
)

type ReviewDecision string

const (
	DecisionPending           ReviewDecision = "PENDING"
	DecisionRevisionsRequired ReviewDecision = "REVISIONS_REQUIRED"
	DecisionApproved          ReviewDecision = "APPROVED"
)

type ReviewFinding struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type FindingResponse struct {
	FindingID string    `json:"finding_id"`
	Response  string    `json:"response"`
	ActorID   string    `json:"actor_id"`
	RequestID string    `json:"request_id,omitempty"`
	At        time.Time `json:"at"`
}

type ReviewRound struct {
	ID                string            `json:"id"`
	PermitID          string            `json:"permit_id"`
	Sequence          int               `json:"sequence"`
	ReviewerID        string            `json:"reviewer_id"`
	Decision          ReviewDecision    `json:"decision"`
	Findings          []ReviewFinding   `json:"findings"`
	Responses         []FindingResponse `json:"responses"`
	SubmittedRevision int64             `json:"submitted_revision"`
	AssignedAt        time.Time         `json:"assigned_at"`
	AssignedBy        string            `json:"assigned_by,omitempty"`
	AssignRequestID   string            `json:"assign_request_id,omitempty"`
	DecisionBy        string            `json:"decision_by,omitempty"`
	DecisionRequestID string            `json:"decision_request_id,omitempty"`
	DecisionReason    string            `json:"decision_reason,omitempty"`
	DecidedAt         *time.Time        `json:"decided_at,omitempty"`
}

func NewReviewRound(id, permitID, reviewer string, sequence int, revision int64, now time.Time) (*ReviewRound, error) {
	if strings.TrimSpace(reviewer) == "" {
		return nil, NewValidation("REVIEWER_REQUIRED", "必须指定审核员", nil)
	}
	return &ReviewRound{ID: id, PermitID: permitID, Sequence: sequence, ReviewerID: reviewer, Decision: DecisionPending, SubmittedRevision: revision, AssignedAt: now.UTC()}, nil
}

func (r *ReviewRound) Decide(decision ReviewDecision, findings []ReviewFinding, reviewer string, now time.Time) error {
	if r.Decision != DecisionPending {
		return NewConflict("REVIEW_DECIDED", "本轮审核已经形成决定")
	}
	if r.ReviewerID != reviewer {
		return NewConflict("REVIEWER_MISMATCH", "只有获分派的审核员可以作出决定")
	}
	if decision != DecisionApproved && decision != DecisionRevisionsRequired {
		return NewValidation("INVALID_DECISION", "审核决定无效", nil)
	}
	if decision == DecisionRevisionsRequired && len(findings) == 0 {
		return NewValidation("FINDINGS_REQUIRED", "退回整改时至少需要一个问题项", nil)
	}
	seen := map[string]bool{}
	for _, f := range findings {
		if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.Message) == "" {
			return NewValidation("FINDING_INCOMPLETE", "审核问题标识和内容不能为空", nil)
		}
		if seen[f.ID] {
			return NewValidation("FINDING_DUPLICATE", "审核问题标识不能重复", nil)
		}
		seen[f.ID] = true
	}
	t := now.UTC()
	r.Decision = decision
	r.Findings = append([]ReviewFinding(nil), findings...)
	r.DecidedAt = &t
	return nil
}

func (r *ReviewRound) Respond(responses []FindingResponse, actor string, now time.Time) error {
	return r.RespondWithRequest(responses, actor, "", now)
}

func (r *ReviewRound) RespondWithRequest(responses []FindingResponse, actor, requestID string, now time.Time) error {
	if r.Decision != DecisionRevisionsRequired {
		return NewConflict("RESPONSES_NOT_ALLOWED", "当前审核轮次不需要整改回应")
	}
	if len(responses) == 0 {
		return NewValidation("RESPONSES_REQUIRED", "至少需要一项整改回应", nil)
	}
	allowed := map[string]bool{}
	for _, f := range r.Findings {
		allowed[f.ID] = true
	}
	seen := map[string]bool{}
	for i := range responses {
		if !allowed[responses[i].FindingID] {
			return NewValidation("UNKNOWN_FINDING", "整改回应引用了不存在的问题项", nil)
		}
		if strings.TrimSpace(responses[i].Response) == "" {
			return NewValidation("EMPTY_RESPONSE", "整改回应不能为空", nil)
		}
		if seen[responses[i].FindingID] {
			return NewValidation("DUPLICATE_RESPONSE", "同一问题项不能重复回应", nil)
		}
		seen[responses[i].FindingID] = true
		responses[i].ActorID = actor
		responses[i].RequestID = requestID
		responses[i].At = now.UTC()
	}
	positions := make(map[string]int, len(r.Responses))
	for i := range r.Responses {
		positions[r.Responses[i].FindingID] = i
	}
	for _, response := range responses {
		if i, ok := positions[response.FindingID]; ok {
			r.Responses[i] = response
			continue
		}
		positions[response.FindingID] = len(r.Responses)
		r.Responses = append(r.Responses, response)
	}
	return nil
}

type FindingClosureSummary struct {
	TotalCount           int      `json:"total_count"`
	RespondedCount       int      `json:"responded_count"`
	UnansweredFindingIDs []string `json:"unanswered_finding_ids"`
	AllClosed            bool     `json:"all_closed"`
}

func (r *ReviewRound) ClosureSummary() FindingClosureSummary {
	answered := map[string]bool{}
	for _, response := range r.Responses {
		answered[response.FindingID] = strings.TrimSpace(response.Response) != ""
	}
	summary := FindingClosureSummary{TotalCount: len(r.Findings), UnansweredFindingIDs: []string{}}
	for _, finding := range r.Findings {
		if answered[finding.ID] {
			summary.RespondedCount++
		} else {
			summary.UnansweredFindingIDs = append(summary.UnansweredFindingIDs, finding.ID)
		}
	}
	summary.AllClosed = summary.TotalCount > 0 && summary.RespondedCount == summary.TotalCount
	return summary
}

func (r *ReviewRound) AllFindingsAnswered() bool {
	return r.ClosureSummary().AllClosed
}
