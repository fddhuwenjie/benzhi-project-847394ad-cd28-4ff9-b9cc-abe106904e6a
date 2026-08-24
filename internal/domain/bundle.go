package domain

type PermitBundle struct {
	Permit               *WorkPermit           `json:"permit"`
	Reviews              []ReviewRound         `json:"reviews"`
	Closure              *ClosureEvidence      `json:"closure,omitempty"`
	Closures             []ClosureEvidence     `json:"closure_versions,omitempty"`
	ClosureVerifications []ClosureVerification `json:"closure_verifications,omitempty"`
	Transitions          []TransitionRecord    `json:"transitions"`
}

func (b *PermitBundle) LatestReview() *ReviewRound {
	if len(b.Reviews) == 0 {
		return nil
	}
	return &b.Reviews[len(b.Reviews)-1]
}
