package domain

import "time"

type TransitionRecord struct {
	ID         string       `json:"id"`
	PermitID   string       `json:"permit_id"`
	FromStatus PermitStatus `json:"from_status"`
	ToStatus   PermitStatus `json:"to_status"`
	ActorID    string       `json:"actor_id"`
	RequestID  string       `json:"request_id"`
	Reason     string       `json:"reason,omitempty"`
	OccurredAt time.Time    `json:"occurred_at"`
}

func NewTransition(id, permitID string, from, to PermitStatus, actor, requestID, reason string, now time.Time) (TransitionRecord, error) {
	if from != "" {
		if err := RequireTransition(from, to); err != nil {
			return TransitionRecord{}, err
		}
	}
	return TransitionRecord{ID: id, PermitID: permitID, FromStatus: from, ToStatus: to, ActorID: actor, RequestID: requestID, Reason: reason, OccurredAt: now.UTC()}, nil
}
