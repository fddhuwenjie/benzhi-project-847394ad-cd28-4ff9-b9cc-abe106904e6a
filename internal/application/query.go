package application

import (
	"context"
	"sort"
	"strconv"

	"confinedpermit/internal/domain"
)

func (s *Service) GetPermit(ctx context.Context, id string) (PermitView, error) {
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return PermitView{}, err
	}
	return viewOf(b, false), nil
}

func (s *Service) PreflightPermit(ctx context.Context, id string) (domain.PreflightReport, error) {
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.PreflightReport{}, err
	}
	if b.Permit.Status != domain.StatusDraft && b.Permit.Status != domain.StatusRevisionsRequired {
		return domain.PreflightReport{}, domain.NewConflict("PREFLIGHT_NOT_ALLOWED", "当前状态不允许执行许可草稿预检")
	}
	return domain.Preflight(b.Permit, s.now()), nil
}

func (s *Service) ListQueue(ctx context.Context, query QueueQuery) (QueueView, error) {
	if err := validateQueueQuery(&query); err != nil {
		return QueueView{}, err
	}
	repo, ok := s.repo.(CollectionRepository)
	if !ok {
		return QueueView{}, domain.NewInternal("存储未提供许可集合快照")
	}
	bundles, err := s.collectionSnapshot(ctx, repo)
	if err != nil {
		return QueueView{}, err
	}
	now := s.now()
	wanted := map[domain.PermitStatus]bool{}
	for _, status := range query.Statuses {
		wanted[status] = true
	}
	items := make([]QueueItem, 0, len(bundles))
	for _, bundle := range bundles {
		permit := bundle.Permit
		if len(wanted) > 0 && !wanted[permit.Status] {
			continue
		}
		if query.SpaceID != "" && permit.SpaceID != query.SpaceID {
			continue
		}
		if query.PlannedStartFrom != nil && permit.PlannedEnd.Before(*query.PlannedStartFrom) {
			continue
		}
		if query.PlannedEndTo != nil && permit.PlannedStart.After(*query.PlannedEndTo) {
			continue
		}
		latestReviewer := ""
		if review := bundle.LatestReview(); review != nil {
			latestReviewer = review.ReviewerID
		}
		if query.ReviewerID != "" && latestReviewer != query.ReviewerID {
			continue
		}
		items = append(items, QueueItem{
			PermitID: permit.ID, SpaceID: permit.SpaceID, Status: permit.Status, Revision: permit.Revision,
			PlannedStart: permit.PlannedStart, PlannedEnd: permit.PlannedEnd, UpdatedAt: permit.UpdatedAt,
			LatestReviewerID: latestReviewer, NextAction: domain.NextAction(bundle), Timing: domain.TimingFlag(permit, now),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		return items[i].PermitID < items[j].PermitID
	})
	summary := QueueSummary{StatusCounts: map[domain.PermitStatus]int{}}
	for _, item := range items {
		summary.StatusCounts[item.Status]++
		if item.Timing == "OVERDUE" {
			summary.OverdueCount++
		}
	}
	filter := filterDigest(struct {
		Statuses   []domain.PermitStatus `json:"statuses"`
		SpaceID    string                `json:"space_id"`
		ReviewerID string                `json:"reviewer_id"`
		From       any                   `json:"from"`
		To         any                   `json:"to"`
	}{query.Statuses, query.SpaceID, query.ReviewerID, query.PlannedStartFrom, query.PlannedEndTo})
	offset := 0
	if query.Cursor != "" {
		offset, err = decodeCursor(query.Cursor, "permit-queue", filter)
		if err != nil {
			return QueueView{}, err
		}
		if offset > len(items) {
			return QueueView{}, domain.NewValidation("CURSOR_INVALID", "cursor 已超出当前查询结果", nil)
		}
	}
	end := offset + query.Limit
	if end > len(items) {
		end = len(items)
	}
	page := append([]QueueItem(nil), items[offset:end]...)
	if page == nil {
		page = []QueueItem{}
	}
	view := QueueView{Items: page, Summary: summary, EvaluatedAt: now}
	if end < len(items) {
		view.NextCursor = encodeCursor("permit-queue", filter, end)
	}
	return view, nil
}

func validateQueueQuery(query *QueueQuery) error {
	for _, status := range query.Statuses {
		if !status.Valid() {
			return domain.NewValidation("STATUS_INVALID", "status 包含未知许可状态", nil)
		}
	}
	if query.PlannedStartFrom != nil && query.PlannedEndTo != nil && query.PlannedEndTo.Before(*query.PlannedStartFrom) {
		return domain.NewValidation("TIME_RANGE_INVALID", "计划时段结束不能早于开始", nil)
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return domain.NewValidation("LIMIT_INVALID", "limit 必须为 1 至 100", nil)
	}
	if query.ReviewerID != "" {
		if err := validateIdentifier("reviewer_id", query.ReviewerID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GetTimeline(ctx context.Context, id string) (TimelineView, error) {
	return s.QueryTimeline(ctx, id, TimelineQuery{})
}

func (s *Service) QueryTimeline(ctx context.Context, id string, query TimelineQuery) (TimelineView, error) {
	if err := validateTimelineQuery(&query); err != nil {
		return TimelineView{}, err
	}
	bundle, err := s.repo.Get(ctx, id)
	if err != nil {
		return TimelineView{}, err
	}
	events := projectAuditEvents(bundle)
	evidenceRequests := map[int]string{}
	for _, evidence := range closureVersions(bundle) {
		evidenceRequests[evidence.Version] = evidence.RequestID
	}
	requestMatched := query.RequestID == ""
	matched := make([]domain.AuditEvent, 0, len(events))
	for _, event := range events {
		requestOK := query.RequestID == "" || event.RequestID == query.RequestID || (event.EventType == "CLOSURE_VERIFICATION" && evidenceRequests[event.EvidenceVersion] == query.RequestID)
		if requestOK && query.RequestID != "" {
			requestMatched = true
		}
		if !requestOK || (query.ActorID != "" && event.ActorID != query.ActorID && event.ReviewerID != query.ActorID) {
			continue
		}
		if query.FromStatus != "" && event.FromStatus != query.FromStatus {
			continue
		}
		if query.ToStatus != "" && event.ToStatus != query.ToStatus {
			continue
		}
		if query.OccurredFrom != nil && event.OccurredAt.Before(*query.OccurredFrom) {
			continue
		}
		if query.OccurredTo != nil && event.OccurredAt.After(*query.OccurredTo) {
			continue
		}
		matched = append(matched, event)
	}
	if !requestMatched {
		return TimelineView{}, domain.NewNotFoundCode("REQUEST_ID_NOT_FOUND", "该许可下不存在匹配 request_id 的审计记录")
	}
	filter := filterDigest(struct {
		PermitID, ActorID, RequestID string
		FromStatus, ToStatus         domain.PermitStatus
		From, To                     any
	}{id, query.ActorID, query.RequestID, query.FromStatus, query.ToStatus, query.OccurredFrom, query.OccurredTo})
	offset := 0
	if query.Cursor != "" {
		offset, err = decodeCursor(query.Cursor, "permit-timeline:"+id, filter)
		if err != nil {
			return TimelineView{}, err
		}
		if offset > len(matched) {
			return TimelineView{}, domain.NewValidation("CURSOR_INVALID", "cursor 已超出当前查询结果", nil)
		}
	}
	end := offset + query.Limit
	if end > len(matched) {
		end = len(matched)
	}
	page := append([]domain.AuditEvent(nil), matched[offset:end]...)
	if page == nil {
		page = []domain.AuditEvent{}
	}
	view := TimelineView{
		PermitID: id, Status: bundle.Permit.Status, CurrentStatus: bundle.Permit.Status, Revision: bundle.Permit.Revision,
		Transitions: append([]domain.TransitionRecord(nil), bundle.Transitions...), Reviews: append([]domain.ReviewRound(nil), bundle.Reviews...), Closure: bundle.Closure,
		TotalCount: len(events), MatchedCount: len(matched), Events: page, EvaluatedAt: s.now(),
	}
	if end < len(matched) {
		view.NextCursor = encodeCursor("permit-timeline:"+id, filter, end)
	}
	return view, nil
}

func validateTimelineQuery(query *TimelineQuery) error {
	if query.ActorID != "" {
		if err := validateIdentifier("actor_id", query.ActorID); err != nil {
			return err
		}
	}
	if query.RequestID != "" {
		if err := validateIdentifier("request_id", query.RequestID); err != nil {
			return err
		}
	}
	if query.FromStatus != "" && !query.FromStatus.Valid() {
		return domain.NewValidation("FROM_STATUS_INVALID", "from_status 不是有效许可状态", nil)
	}
	if query.ToStatus != "" && !query.ToStatus.Valid() {
		return domain.NewValidation("TO_STATUS_INVALID", "to_status 不是有效许可状态", nil)
	}
	if query.OccurredFrom != nil && query.OccurredTo != nil && query.OccurredTo.Before(*query.OccurredFrom) {
		return domain.NewValidation("TIME_RANGE_INVALID", "发生时间结束不能早于开始", nil)
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return domain.NewValidation("LIMIT_INVALID", "limit 必须为 1 至 100", nil)
	}
	return nil
}

func projectAuditEvents(bundle *domain.PermitBundle) []domain.AuditEvent {
	var events []domain.AuditEvent
	order := 0
	for _, transition := range bundle.Transitions {
		order++
		events = append(events, domain.AuditEvent{ID: transition.ID, EventType: "STATUS_TRANSITION", OccurredAt: transition.OccurredAt, Sequence: order, ActorID: transition.ActorID, RequestID: transition.RequestID, FromStatus: transition.FromStatus, ToStatus: transition.ToStatus, Note: transition.Reason})
	}
	for _, review := range bundle.Reviews {
		order++
		events = append(events, domain.AuditEvent{ID: review.ID + ":assigned", EventType: "REVIEW_ASSIGNED", OccurredAt: review.AssignedAt, Sequence: order, ActorID: review.AssignedBy, RequestID: review.AssignRequestID, ReviewID: review.ID, ReviewSequence: review.Sequence, ReviewerID: review.ReviewerID, Decision: string(review.Decision)})
		if review.DecidedAt != nil {
			order++
			events = append(events, domain.AuditEvent{ID: review.ID + ":decision", EventType: "REVIEW_DECIDED", OccurredAt: *review.DecidedAt, Sequence: order, ActorID: review.DecisionBy, RequestID: review.DecisionRequestID, ReviewID: review.ID, ReviewSequence: review.Sequence, ReviewerID: review.ReviewerID, Decision: string(review.Decision), Note: review.DecisionReason})
		}
		for _, response := range review.Responses {
			order++
			events = append(events, domain.AuditEvent{ID: review.ID + ":response:" + response.FindingID, EventType: "FINDING_RESPONDED", OccurredAt: response.At, Sequence: order, ActorID: response.ActorID, RequestID: response.RequestID, ReviewID: review.ID, ReviewSequence: review.Sequence, ReviewerID: review.ReviewerID, FindingID: response.FindingID, Note: response.Response})
		}
	}
	for _, evidence := range closureVersions(bundle) {
		order++
		events = append(events, domain.AuditEvent{ID: "closure:" + strconv.Itoa(evidence.Version), EventType: "CLOSURE_EVIDENCE_SUBMITTED", OccurredAt: evidence.SubmittedAt, Sequence: order, ActorID: evidence.SubmittedBy, RequestID: evidence.RequestID, EvidenceVersion: evidence.Version})
	}
	for _, verification := range bundle.ClosureVerifications {
		order++
		events = append(events, domain.AuditEvent{ID: verification.ID, EventType: "CLOSURE_VERIFICATION", OccurredAt: verification.OccurredAt, Sequence: order, ActorID: verification.ActorID, RequestID: verification.RequestID, EvidenceVersion: verification.EvidenceVersion, Decision: string(verification.Decision), Note: verification.Note, Issues: verification.Issues})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].OccurredAt.Before(events[j].OccurredAt)
		}
		if events[i].Sequence != events[j].Sequence {
			return events[i].Sequence < events[j].Sequence
		}
		return events[i].ID < events[j].ID
	})
	for i := range events {
		events[i].Sequence = i + 1
	}
	return events
}

func closureVersions(bundle *domain.PermitBundle) []domain.ClosureEvidence {
	if len(bundle.Closures) > 0 {
		return bundle.Closures
	}
	if bundle.Closure != nil {
		copy := *bundle.Closure
		if copy.Version == 0 {
			copy.Version = 1
		}
		return []domain.ClosureEvidence{copy}
	}
	return nil
}
