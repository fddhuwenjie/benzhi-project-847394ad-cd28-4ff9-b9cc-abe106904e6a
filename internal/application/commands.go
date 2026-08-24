package application

import "confinedpermit/internal/domain"

type RequestMeta struct {
	ActorID          string `json:"actor_id"`
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreatePermitCommand struct {
	Meta  RequestMeta      `json:"meta"`
	Draft domain.DraftData `json:"draft"`
}

type RevisePermitCommand struct {
	Meta  RequestMeta      `json:"meta"`
	Draft domain.DraftData `json:"draft"`
}

type ActionCommand struct {
	Meta RequestMeta `json:"meta"`
}

type ActivatePermitCommand struct {
	Meta             RequestMeta                   `json:"meta"`
	SiteVerification *domain.SiteVerificationInput `json:"site_verification,omitempty"`
}

type AssignReviewCommand struct {
	Meta       RequestMeta `json:"meta"`
	ReviewerID string      `json:"reviewer_id"`
}

type DecideReviewCommand struct {
	Meta     RequestMeta            `json:"meta"`
	Decision domain.ReviewDecision  `json:"decision"`
	Findings []domain.ReviewFinding `json:"findings"`
	Reason   string                 `json:"reason"`
}

type RespondReviewCommand struct {
	Meta      RequestMeta              `json:"meta"`
	Responses []domain.FindingResponse `json:"responses"`
}

type ClosureCommand struct {
	Meta               RequestMeta `json:"meta"`
	PersonnelCleared   bool        `json:"personnel_cleared"`
	ToolsAccounted     bool        `json:"tools_accounted"`
	IsolationsRestored bool        `json:"isolations_restored"`
	PhotoRefs          []string    `json:"photo_refs"`
}

type VerifyClosureCommand struct {
	Meta            RequestMeta            `json:"meta"`
	Decision        domain.ClosureDecision `json:"decision"`
	Note            string                 `json:"note"`
	Issues          []domain.Issue         `json:"issues,omitempty"`
	EvidenceVersion int                    `json:"evidence_version,omitempty"`
}
