package httpapi

import (
	"log/slog"
	"net/http"

	"confinedpermit/internal/application"
)

type API struct {
	service *application.Service
	logger  *slog.Logger
}

func New(service *application.Service, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	a := &API{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/permits", a.CreatePermitHandler)
	mux.HandleFunc("GET /api/v1/permits", a.ListPermitsHandler)
	mux.HandleFunc("GET /api/v1/permits/{permit_id}", a.GetPermitHandler)
	mux.HandleFunc("GET /api/v1/permits/{permit_id}/preflight", a.PreflightPermitHandler)
	mux.HandleFunc("PATCH /api/v1/permits/{permit_id}", a.RevisePermitHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/submit", a.SubmitPermitHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/reviews/assign", a.AssignReviewHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/reviews/decision", a.DecideReviewHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/reviews/responses", a.RespondReviewHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/activate", a.ActivatePermitHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/closure", a.SubmitClosureHandler)
	mux.HandleFunc("POST /api/v1/permits/{permit_id}/closure/verify", a.VerifyClosureHandler)
	mux.HandleFunc("GET /api/v1/permits/{permit_id}/timeline", a.GetTimelineHandler)
	return a.requestLog(mux)
}
