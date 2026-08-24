package application

import (
	"context"
	"strings"

	"confinedpermit/internal/domain"
)

func (s *Service) AssignReview(ctx context.Context, id string, cmd AssignReviewCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	if err := validateIdentifier("reviewer_id", cmd.ReviewerID); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "assign-review", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.repo.Mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		if b.Permit.Status != domain.StatusPendingReview {
			return domain.NewConflict("ASSIGN_NOT_ALLOWED", "只有待审核许可可以分派审核员")
		}
		if last := b.LatestReview(); last != nil && last.Decision == domain.DecisionPending {
			return domain.NewConflict("REVIEW_ALREADY_ASSIGNED", "当前审核轮次已经分派")
		}
		r, err := domain.NewReviewRound(newID("review"), id, cmd.ReviewerID, len(b.Reviews)+1, b.Permit.Revision, s.now())
		if err != nil {
			return err
		}
		r.AssignedBy = cmd.Meta.ActorID
		r.AssignRequestID = cmd.Meta.RequestID
		b.Reviews = append(b.Reviews, *r)
		b.Permit.UpdatedAt = s.now()
		return nil
	})
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(saved, replay), nil
}

func (s *Service) DecideReview(ctx context.Context, id string, cmd DecideReviewCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "decide-review", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.repo.Mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		if b.Permit.Status != domain.StatusPendingReview {
			return domain.NewConflict("DECISION_NOT_ALLOWED", "许可当前不处于待审核状态")
		}
		r := b.LatestReview()
		if r == nil || r.Decision != domain.DecisionPending {
			return domain.NewConflict("REVIEW_NOT_ASSIGNED", "没有可决定的审核轮次")
		}
		now := s.now()
		if err := r.Decide(cmd.Decision, cmd.Findings, cmd.Meta.ActorID, now); err != nil {
			return err
		}
		r.DecisionBy = cmd.Meta.ActorID
		r.DecisionRequestID = cmd.Meta.RequestID
		from := b.Permit.Status
		var to domain.PermitStatus
		var reason string
		switch cmd.Decision {
		case domain.DecisionRevisionsRequired:
			to = domain.StatusRevisionsRequired
			reason = strings.TrimSpace(cmd.Reason)
			if reason == "" {
				reason = "审核发现风险控制问题，要求整改"
			}
		case domain.DecisionApproved:
			if issues := domain.ValidateForSubmission(b.Permit, now); len(issues) > 0 {
				return domain.NewValidation("APPROVAL_REJECTED", "批准时安全控制已经失效", issues)
			}
			to = domain.StatusApproved
			reason = strings.TrimSpace(cmd.Reason)
			if reason == "" {
				reason = "审核员确认风险控制有效"
			}
			b.Permit.ApprovedSnapshot = domain.FreezeApproval(b.Permit, cmd.Meta.ActorID, now)
		default:
			return domain.NewValidation("INVALID_DECISION", "审核决定无效", nil)
		}
		r.DecisionReason = reason
		if err := b.Permit.Transition(to, now); err != nil {
			return err
		}
		tr, err := domain.NewTransition(newID("transition"), id, from, to, cmd.Meta.ActorID, cmd.Meta.RequestID, reason, now)
		if err != nil {
			return err
		}
		b.Transitions = append(b.Transitions, tr)
		return nil
	})
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(saved, replay), nil
}

func (s *Service) RespondToReview(ctx context.Context, id string, cmd RespondReviewCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "respond-review", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.repo.Mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		if b.Permit.Status != domain.StatusRevisionsRequired {
			return domain.NewConflict("RESPONSES_NOT_ALLOWED", "许可当前不需要整改回应")
		}
		if b.Permit.OwnerID != "" && b.Permit.OwnerID != cmd.Meta.ActorID {
			return domain.NewConflict("OWNER_MISMATCH", "只有许可作业负责人可以提交整改回应")
		}
		r := b.LatestReview()
		if r == nil {
			return domain.NewConflict("REVIEW_NOT_FOUND", "没有可回应的审核轮次")
		}
		if err := r.RespondWithRequest(cmd.Responses, cmd.Meta.ActorID, cmd.Meta.RequestID, s.now()); err != nil {
			return err
		}
		b.Permit.UpdatedAt = s.now()
		return nil
	})
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(saved, replay), nil
}
