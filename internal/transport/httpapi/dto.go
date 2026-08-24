package httpapi

import (
	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
)

type metaRequest struct {
	ActorID          string `json:"actor_id"`
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision,omitempty"`
}

func (m metaRequest) applicationMeta() application.RequestMeta {
	return application.RequestMeta{ActorID: m.ActorID, RequestID: m.RequestID, ExpectedRevision: m.ExpectedRevision}
}

type createRequest struct {
	metaRequest
	Draft domain.DraftData `json:"draft"`
}

type reviseRequest struct {
	metaRequest
	Draft domain.DraftData `json:"draft"`
}

type actionRequest struct{ metaRequest }

type activateRequest struct {
	metaRequest
	SiteVerification *domain.SiteVerificationInput `json:"site_verification,omitempty"`
}

type assignReviewRequest struct {
	metaRequest
	ReviewerID string `json:"reviewer_id"`
}

type decideReviewRequest struct {
	metaRequest
	Decision domain.ReviewDecision  `json:"decision"`
	Findings []domain.ReviewFinding `json:"findings,omitempty"`
	Reason   string                 `json:"reason,omitempty"`
}

type respondReviewRequest struct {
	metaRequest
	Responses []findingResponseRequest `json:"responses"`
}

type findingResponseRequest struct {
	FindingID string `json:"finding_id"`
	Response  string `json:"response"`
}

type closureRequest struct {
	metaRequest
	PersonnelCleared   bool     `json:"personnel_cleared"`
	ToolsAccounted     bool     `json:"tools_accounted"`
	IsolationsRestored bool     `json:"isolations_restored"`
	PhotoRefs          []string `json:"photo_refs"`
}

type verifyClosureRequest struct {
	metaRequest
	Decision        domain.ClosureDecision `json:"decision,omitempty"`
	Note            string                 `json:"note,omitempty"`
	Issues          []domain.Issue         `json:"issues,omitempty"`
	EvidenceVersion int                    `json:"evidence_version,omitempty"`
}
