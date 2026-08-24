package httpapi

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
)

func (a *API) GetPermitHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	result, err := a.service.GetPermit(r.Context(), id)
	if err != nil {
		writeError(w, err, "")
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) PreflightPermitHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	result, err := a.service.PreflightPermit(r.Context(), id)
	if err != nil {
		writeError(w, err, "")
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) ListPermitsHandler(w http.ResponseWriter, r *http.Request) {
	query, err := parseQueueQuery(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	result, err := a.service.ListQueue(r.Context(), query)
	if err != nil {
		writeError(w, err, "")
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) GetTimelineHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	query, err := parseTimelineQuery(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	result, err := a.service.QueryTimeline(r.Context(), id, query)
	if err != nil {
		writeError(w, err, "")
		return
	}
	writeData(w, http.StatusOK, result)
}

func parseQueueQuery(r *http.Request) (application.QueueQuery, error) {
	allowed := map[string]bool{"status": true, "space_id": true, "reviewer_id": true, "planned_start_from": true, "planned_end_to": true, "limit": true, "cursor": true}
	if err := rejectUnknownQuery(r, allowed); err != nil {
		return application.QueueQuery{}, err
	}
	values := r.URL.Query()
	if err := rejectRepeatedQuery(values, map[string]bool{"status": true}); err != nil {
		return application.QueueQuery{}, err
	}
	var statuses []domain.PermitStatus
	for _, raw := range values["status"] {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				return application.QueueQuery{}, queryError("STATUS_INVALID", "status 不能为空")
			}
			statuses = append(statuses, domain.PermitStatus(value))
		}
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i] < statuses[j] })
	from, err := queryTime(values.Get("planned_start_from"), "planned_start_from")
	if err != nil {
		return application.QueueQuery{}, err
	}
	to, err := queryTime(values.Get("planned_end_to"), "planned_end_to")
	if err != nil {
		return application.QueueQuery{}, err
	}
	limit, err := queryLimit(values.Get("limit"))
	if err != nil {
		return application.QueueQuery{}, err
	}
	return application.QueueQuery{Statuses: statuses, SpaceID: strings.TrimSpace(values.Get("space_id")), ReviewerID: strings.TrimSpace(values.Get("reviewer_id")), PlannedStartFrom: from, PlannedEndTo: to, Limit: limit, Cursor: strings.TrimSpace(values.Get("cursor"))}, nil
}

func parseTimelineQuery(r *http.Request) (application.TimelineQuery, error) {
	allowed := map[string]bool{"actor_id": true, "request_id": true, "from_status": true, "to_status": true, "occurred_from": true, "occurred_to": true, "limit": true, "cursor": true}
	if err := rejectUnknownQuery(r, allowed); err != nil {
		return application.TimelineQuery{}, err
	}
	values := r.URL.Query()
	if err := rejectRepeatedQuery(values, nil); err != nil {
		return application.TimelineQuery{}, err
	}
	from, err := queryTime(values.Get("occurred_from"), "occurred_from")
	if err != nil {
		return application.TimelineQuery{}, err
	}
	to, err := queryTime(values.Get("occurred_to"), "occurred_to")
	if err != nil {
		return application.TimelineQuery{}, err
	}
	limit, err := queryLimit(values.Get("limit"))
	if err != nil {
		return application.TimelineQuery{}, err
	}
	return application.TimelineQuery{ActorID: strings.TrimSpace(values.Get("actor_id")), RequestID: strings.TrimSpace(values.Get("request_id")), FromStatus: domain.PermitStatus(strings.TrimSpace(values.Get("from_status"))), ToStatus: domain.PermitStatus(strings.TrimSpace(values.Get("to_status"))), OccurredFrom: from, OccurredTo: to, Limit: limit, Cursor: strings.TrimSpace(values.Get("cursor"))}, nil
}

func rejectRepeatedQuery(values map[string][]string, repeatable map[string]bool) error {
	for key, entries := range values {
		if len(entries) > 1 && !repeatable[key] {
			return queryError("QUERY_PARAMETER_DUPLICATE", "查询参数 "+key+" 不能重复")
		}
	}
	return nil
}

func rejectUnknownQuery(r *http.Request, allowed map[string]bool) error {
	for key := range r.URL.Query() {
		if !allowed[key] {
			return queryError("QUERY_PARAMETER_INVALID", "未知查询参数 "+key)
		}
	}
	return nil
}

func queryTime(raw, field string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, queryError("TIME_INVALID", field+" 必须为 RFC3339 时间")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func queryLimit(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, queryError("LIMIT_INVALID", "limit 必须为整数")
	}
	return limit, nil
}

func queryError(code, message string) error {
	return &protocolError{Status: http.StatusBadRequest, Code: code, Message: message}
}
