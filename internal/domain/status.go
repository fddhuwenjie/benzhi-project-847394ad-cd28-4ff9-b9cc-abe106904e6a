package domain

import "fmt"

type PermitStatus string

const (
	StatusDraft             PermitStatus = "DRAFT"
	StatusPendingReview     PermitStatus = "PENDING_REVIEW"
	StatusRevisionsRequired PermitStatus = "REVISIONS_REQUIRED"
	StatusApproved          PermitStatus = "APPROVED"
	StatusActive            PermitStatus = "ACTIVE"
	StatusClosureReview     PermitStatus = "CLOSURE_REVIEW"
	StatusClosed            PermitStatus = "CLOSED"
)

var legalTransitions = map[PermitStatus]map[PermitStatus]bool{
	StatusDraft:             {StatusPendingReview: true},
	StatusPendingReview:     {StatusRevisionsRequired: true, StatusApproved: true},
	StatusRevisionsRequired: {StatusPendingReview: true},
	StatusApproved:          {StatusActive: true},
	StatusActive:            {StatusClosureReview: true},
	StatusClosureReview:     {StatusClosed: true},
}

func (s PermitStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusPendingReview, StatusRevisionsRequired, StatusApproved, StatusActive, StatusClosureReview, StatusClosed:
		return true
	default:
		return false
	}
}

func CanTransition(from, to PermitStatus) bool { return legalTransitions[from][to] }

func RequireTransition(from, to PermitStatus) error {
	if !CanTransition(from, to) {
		return NewConflict("ILLEGAL_TRANSITION", fmt.Sprintf("许可不能从 %s 转换为 %s", from, to))
	}
	return nil
}
