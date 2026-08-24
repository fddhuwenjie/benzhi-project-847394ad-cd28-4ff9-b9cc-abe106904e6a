package httpapi

import (
	"net/http"

	"confinedpermit/internal/application"
)

func (a *API) ActivatePermitHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in activateRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	result, err := a.service.ActivatePermit(r.Context(), id, application.ActivatePermitCommand{Meta: in.applicationMeta(), SiteVerification: in.SiteVerification})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) SubmitClosureHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in closureRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	cmd := application.ClosureCommand{Meta: in.applicationMeta(), PersonnelCleared: in.PersonnelCleared, ToolsAccounted: in.ToolsAccounted, IsolationsRestored: in.IsolationsRestored, PhotoRefs: in.PhotoRefs}
	result, err := a.service.SubmitClosure(r.Context(), id, cmd)
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) VerifyClosureHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in verifyClosureRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	result, err := a.service.VerifyClosure(r.Context(), id, application.VerifyClosureCommand{Meta: in.applicationMeta(), Decision: in.Decision, Note: in.Note, Issues: in.Issues, EvidenceVersion: in.EvidenceVersion})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}
