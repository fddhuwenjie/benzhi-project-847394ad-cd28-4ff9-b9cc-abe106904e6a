package application

import (
	"context"

	"confinedpermit/internal/domain"
)

func (s *Service) CreatePermit(ctx context.Context, cmd CreatePermitCommand) (PermitView, error) {
	key, err := makeKey("permits", "create", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	if issues := domain.ValidateDraftShape(cmd.Draft); len(issues) > 0 {
		return PermitView{}, domain.NewValidation("DRAFT_INVALID", "许可草稿集合格式无效", issues)
	}
	now := s.now()
	p := domain.NewPermit(newID("permit"), cmd.Draft, now)
	p.OwnerID = cmd.Meta.ActorID
	tr, err := domain.NewTransition(newID("transition"), p.ID, "", domain.StatusDraft, cmd.Meta.ActorID, cmd.Meta.RequestID, "创建许可草稿", now)
	if err != nil {
		return PermitView{}, err
	}
	b := &domain.PermitBundle{Permit: p, Transitions: []domain.TransitionRecord{tr}}
	saved, replay, err := s.create(ctx, b, key)
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(saved, replay), nil
}

func (s *Service) RevisePermit(ctx context.Context, id string, cmd RevisePermitCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "revise", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	if issues := domain.ValidateDraftShape(cmd.Draft); len(issues) > 0 {
		return PermitView{}, domain.NewValidation("DRAFT_INVALID", "许可草稿集合格式无效", issues)
	}
	saved, replay, err := s.mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		return b.Permit.ReplaceDraft(cmd.Draft, s.now())
	})
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(saved, replay), nil
}

func (s *Service) SubmitPermit(ctx context.Context, id string, cmd ActionCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "submit", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		from := b.Permit.Status
		if from != domain.StatusDraft && from != domain.StatusRevisionsRequired {
			return domain.NewConflict("SUBMIT_NOT_ALLOWED", "当前状态不允许提交审核")
		}
		if from == domain.StatusRevisionsRequired {
			r := b.LatestReview()
			if r == nil || !r.AllFindingsAnswered() {
				var issues []domain.Issue
				if r != nil {
					for _, findingID := range r.ClosureSummary().UnansweredFindingIDs {
						issues = append(issues, domain.Issue{Category: "REVIEW_FINDINGS", Code: "FINDING_UNANSWERED", Field: "responses." + findingID, Message: "审核问题 " + findingID + " 尚未回应"})
					}
				}
				return domain.NewValidation("FINDINGS_UNANSWERED", "必须逐项回应上一轮审核问题", issues)
			}
		}
		now := s.now()
		if issues := domain.ValidateForSubmission(b.Permit, now); len(issues) > 0 {
			return domain.NewValidation("SUBMISSION_REJECTED", "许可未通过提交前安全校验", issues)
		}
		if err := b.Permit.Transition(domain.StatusPendingReview, now); err != nil {
			return err
		}
		tr, err := domain.NewTransition(newID("transition"), id, from, domain.StatusPendingReview, cmd.Meta.ActorID, cmd.Meta.RequestID, "提交安全审核", now)
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
