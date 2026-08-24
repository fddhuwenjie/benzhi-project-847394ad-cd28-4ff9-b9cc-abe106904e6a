package application

import (
	"context"

	"confinedpermit/internal/domain"
)

func (s *Service) ActivatePermit(ctx context.Context, id string, cmd ActivatePermitCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "activate", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.repo.Mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		if b.Permit.Status != domain.StatusApproved {
			return domain.NewConflict("ACTIVATION_NOT_ALLOWED", "只有已批准许可可以激活")
		}
		if b.Permit.ApprovedSnapshot == nil {
			return domain.NewConflict("APPROVAL_SNAPSHOT_MISSING", "缺少批准快照")
		}
		now := s.now()
		issues := domain.ValidateActivationWindow(b.Permit, now)
		site, siteIssues := domain.ValidateSiteVerification(b.Permit.ApprovedSnapshot, cmd.SiteVerification, cmd.Meta.ActorID, cmd.Meta.RequestID, now)
		issues = append(issues, siteIssues...)
		if len(issues) > 0 {
			return domain.NewValidation("ACTIVATION_REJECTED", "许可不满足激活条件", issues)
		}
		b.Permit.SiteVerification = site
		if err := b.Permit.Transition(domain.StatusActive, now); err != nil {
			return err
		}
		tr, err := domain.NewTransition(newID("transition"), id, domain.StatusApproved, domain.StatusActive, cmd.Meta.ActorID, cmd.Meta.RequestID, "在计划时段内激活许可", now)
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

func (s *Service) SubmitClosure(ctx context.Context, id string, cmd ClosureCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "submit-closure", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.repo.Mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		initial := b.Permit.Status == domain.StatusActive
		resubmission := b.Permit.Status == domain.StatusClosureReview
		if !initial && !resubmission {
			return domain.NewConflict("CLOSURE_NOT_ALLOWED", "当前状态不允许申报退场证据")
		}
		if resubmission {
			if b.Permit.OwnerID != "" && b.Permit.OwnerID != cmd.Meta.ActorID {
				return domain.NewConflict("OWNER_MISMATCH", "只有许可作业负责人可以重提退场证据")
			}
			latest := latestClosureVerification(b)
			if latest == nil || latest.Decision != domain.ClosureDecisionRejected || latest.EvidenceVersion != latestClosureVersion(b) {
				return domain.NewConflict("CLOSURE_RESUBMISSION_NOT_ALLOWED", "仅可在最新退场证据被退回后重提")
			}
		}
		now := s.now()
		e, issues := domain.NewClosureEvidence(id, cmd.PersonnelCleared, cmd.ToolsAccounted, cmd.IsolationsRestored, cmd.PhotoRefs, cmd.Meta.ActorID, now)
		if len(issues) > 0 {
			return domain.NewValidation("CLOSURE_EVIDENCE_INCOMPLETE", "退场证据不完整，许可保持 ACTIVE", issues)
		}
		e.Version = latestClosureVersion(b) + 1
		e.RequestID = cmd.Meta.RequestID
		if initial {
			if err := b.Permit.Transition(domain.StatusClosureReview, now); err != nil {
				return err
			}
		}
		b.Closures = append(b.Closures, *e)
		b.Closure = e
		if initial {
			tr, err := domain.NewTransition(newID("transition"), id, domain.StatusActive, domain.StatusClosureReview, cmd.Meta.ActorID, cmd.Meta.RequestID, "提交完整退场证据", now)
			if err != nil {
				return err
			}
			b.Transitions = append(b.Transitions, tr)
		} else {
			b.Permit.UpdatedAt = now
		}
		return nil
	})
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(saved, replay), nil
}

func (s *Service) VerifyClosure(ctx context.Context, id string, cmd VerifyClosureCommand) (PermitView, error) {
	if err := requireRevision(cmd.Meta); err != nil {
		return PermitView{}, err
	}
	key, err := makeKey(id, "verify-closure", cmd.Meta, cmd)
	if err != nil {
		return PermitView{}, err
	}
	saved, replay, err := s.repo.Mutate(ctx, id, cmd.Meta.ExpectedRevision, key, func(b *domain.PermitBundle) error {
		if b.Permit.Status != domain.StatusClosureReview {
			return domain.NewConflict("VERIFICATION_NOT_ALLOWED", "许可当前不处于退场核验状态")
		}
		if b.Closure == nil {
			return domain.NewConflict("CLOSURE_EVIDENCE_MISSING", "缺少退场证据")
		}
		now := s.now()
		version := latestClosureVersion(b)
		if cmd.EvidenceVersion != 0 && cmd.EvidenceVersion != version {
			return domain.NewConflict("EVIDENCE_VERSION_CONFLICT", "只能核验最新退场证据版本")
		}
		if previous := latestClosureVerification(b); previous != nil && previous.EvidenceVersion == version {
			if previous.Decision == domain.ClosureDecisionRejected {
				return domain.NewConflict("EVIDENCE_RESUBMISSION_REQUIRED", "最新退场证据已被退回，必须先重提证据")
			}
			return domain.NewConflict("CLOSURE_ALREADY_VERIFIED", "最新退场证据已经核验")
		}
		decision := cmd.Decision
		if decision == "" {
			decision = domain.ClosureDecisionApproved
		}
		verification, err := domain.NewClosureVerification(newID("closure_verification"), version, decision, cmd.Note, cmd.Issues, cmd.Meta.ActorID, cmd.Meta.RequestID, now)
		if err != nil {
			return err
		}
		b.ClosureVerifications = append(b.ClosureVerifications, verification)
		if decision == domain.ClosureDecisionRejected {
			b.Permit.UpdatedAt = now
			return nil
		}
		_ = b.Closure.Verify(cmd.Meta.ActorID, cmd.Note, now)
		if len(b.Closures) > 0 {
			b.Closures[len(b.Closures)-1] = *b.Closure
		}
		if err := b.Permit.Transition(domain.StatusClosed, now); err != nil {
			return err
		}
		tr, err := domain.NewTransition(newID("transition"), id, domain.StatusClosureReview, domain.StatusClosed, cmd.Meta.ActorID, cmd.Meta.RequestID, cmd.Note, now)
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

func latestClosureVersion(b *domain.PermitBundle) int {
	if len(b.Closures) > 0 {
		return b.Closures[len(b.Closures)-1].Version
	}
	if b.Closure != nil {
		if b.Closure.Version > 0 {
			return b.Closure.Version
		}
		return 1
	}
	return 0
}

func latestClosureVerification(b *domain.PermitBundle) *domain.ClosureVerification {
	if len(b.ClosureVerifications) == 0 {
		return nil
	}
	return &b.ClosureVerifications[len(b.ClosureVerifications)-1]
}
