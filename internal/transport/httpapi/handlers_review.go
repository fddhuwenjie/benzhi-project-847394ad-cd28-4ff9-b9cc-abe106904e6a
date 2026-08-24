package httpapi

import (
	"net/http"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
)

func (a *API) AssignReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in assignReviewRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	result, err := a.service.AssignReview(r.Context(), id, application.AssignReviewCommand{Meta: in.applicationMeta(), ReviewerID: in.ReviewerID})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) DecideReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in decideReviewRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	cmd := application.DecideReviewCommand{Meta: in.applicationMeta(), Decision: in.Decision, Findings: in.Findings, Reason: in.Reason}
	result, err := a.service.DecideReview(r.Context(), id, cmd)
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) RespondReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in respondReviewRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	responses := make([]domain.FindingResponse, len(in.Responses))
	for i, response := range in.Responses {
		responses[i] = domain.FindingResponse{FindingID: response.FindingID, Response: response.Response}
	}
	result, err := a.service.RespondToReview(r.Context(), id, application.RespondReviewCommand{Meta: in.applicationMeta(), Responses: responses})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}
