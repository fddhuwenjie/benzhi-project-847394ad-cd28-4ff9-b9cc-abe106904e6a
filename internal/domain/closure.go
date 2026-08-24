package domain

import (
	"strings"
	"time"
)

type ClosureEvidence struct {
	PermitID           string     `json:"permit_id"`
	Version            int        `json:"version"`
	PersonnelCleared   bool       `json:"personnel_cleared"`
	ToolsAccounted     bool       `json:"tools_accounted"`
	IsolationsRestored bool       `json:"isolations_restored"`
	PhotoRefs          []string   `json:"photo_refs"`
	SubmittedBy        string     `json:"submitted_by"`
	RequestID          string     `json:"request_id,omitempty"`
	SubmittedAt        time.Time  `json:"submitted_at"`
	VerifiedBy         string     `json:"verified_by,omitempty"`
	VerificationNote   string     `json:"verification_note,omitempty"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
}

type ClosureDecision string

const (
	ClosureDecisionApproved ClosureDecision = "APPROVED"
	ClosureDecisionRejected ClosureDecision = "REJECTED"
)

type ClosureVerification struct {
	ID              string          `json:"id"`
	EvidenceVersion int             `json:"evidence_version"`
	Decision        ClosureDecision `json:"decision"`
	Note            string          `json:"note,omitempty"`
	Issues          []Issue         `json:"issues,omitempty"`
	ActorID         string          `json:"actor_id"`
	RequestID       string          `json:"request_id"`
	OccurredAt      time.Time       `json:"occurred_at"`
}

func NewClosureEvidence(permitID string, personnel, tools, isolations bool, photos []string, actor string, now time.Time) (*ClosureEvidence, []Issue) {
	var issues []Issue
	if !personnel {
		issues = append(issues, issue("PERSONNEL_NOT_CLEARED", "personnel_cleared", "必须确认所有人员已经撤离"))
	}
	if !tools {
		issues = append(issues, issue("TOOLS_NOT_ACCOUNTED", "tools_accounted", "必须完成工具清点"))
	}
	if !isolations {
		issues = append(issues, issue("ISOLATIONS_NOT_RESTORED", "isolations_restored", "必须确认隔离已经恢复"))
	}
	if len(photos) == 0 {
		issues = append(issues, issue("PHOTOS_REQUIRED", "photo_refs", "至少需要一张现场照片引用"))
	}
	for _, p := range photos {
		if strings.TrimSpace(p) == "" {
			issues = append(issues, issue("PHOTO_REF_EMPTY", "photo_refs", "照片引用不能为空"))
		}
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return &ClosureEvidence{PermitID: permitID, Version: 1, PersonnelCleared: personnel, ToolsAccounted: tools, IsolationsRestored: isolations, PhotoRefs: append([]string(nil), photos...), SubmittedBy: actor, SubmittedAt: now.UTC()}, nil
}

func NewClosureVerification(id string, evidenceVersion int, decision ClosureDecision, note string, issues []Issue, actor, requestID string, now time.Time) (ClosureVerification, error) {
	if evidenceVersion < 1 {
		return ClosureVerification{}, NewValidation("EVIDENCE_VERSION_REQUIRED", "evidence_version 必须大于零", nil)
	}
	if decision != ClosureDecisionApproved && decision != ClosureDecisionRejected {
		return ClosureVerification{}, NewValidation("INVALID_CLOSURE_DECISION", "退场核验决定必须为 APPROVED 或 REJECTED", nil)
	}
	if decision == ClosureDecisionRejected {
		if len(issues) == 0 {
			return ClosureVerification{}, NewValidation("VERIFICATION_ISSUES_REQUIRED", "退回证据时至少需要一个问题项", nil)
		}
		for i, item := range issues {
			if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Field) == "" || strings.TrimSpace(item.Message) == "" {
				return ClosureVerification{}, NewValidation("VERIFICATION_ISSUE_INCOMPLETE", "退回问题的 code、field 和 message 不能为空", nil)
			}
			if issues[i].Category == "" {
				issues[i].Category = issueCategory(item.Field)
			}
		}
	} else if strings.TrimSpace(note) == "" {
		return ClosureVerification{}, NewValidation("VERIFICATION_NOTE_REQUIRED", "关闭核验必须填写说明", nil)
	}
	return ClosureVerification{ID: id, EvidenceVersion: evidenceVersion, Decision: decision, Note: strings.TrimSpace(note), Issues: append([]Issue(nil), issues...), ActorID: actor, RequestID: requestID, OccurredAt: now.UTC()}, nil
}

func (e *ClosureEvidence) Verify(actor, note string, now time.Time) error {
	if e.VerifiedAt != nil {
		return NewConflict("CLOSURE_ALREADY_VERIFIED", "退场证据已经核验")
	}
	if strings.TrimSpace(note) == "" {
		return NewValidation("VERIFICATION_NOTE_REQUIRED", "关闭核验必须填写说明", nil)
	}
	t := now.UTC()
	e.VerifiedBy = actor
	e.VerificationNote = note
	e.VerifiedAt = &t
	return nil
}
