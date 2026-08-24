package httpapi

import (
	"net/http"

	"confinedpermit/internal/application"
)

func (a *API) CreatePermitHandler(w http.ResponseWriter, r *http.Request) {
	var in createRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	result, err := a.service.CreatePermit(r.Context(), application.CreatePermitCommand{Meta: in.applicationMeta(), Draft: in.Draft})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) RevisePermitHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in reviseRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	result, err := a.service.RevisePermit(r.Context(), id, application.RevisePermitCommand{Meta: in.applicationMeta(), Draft: in.Draft})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) SubmitPermitHandler(w http.ResponseWriter, r *http.Request) {
	id, err := permitID(r)
	if err != nil {
		writeError(w, err, "")
		return
	}
	var in actionRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	r.Header.Set("X-Request-ID", in.RequestID)
	result, err := a.service.SubmitPermit(r.Context(), id, application.ActionCommand{Meta: in.applicationMeta()})
	if err != nil {
		writeError(w, err, in.RequestID)
		return
	}
	writeData(w, http.StatusOK, result)
}
