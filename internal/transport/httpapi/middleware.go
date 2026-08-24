package httpapi

import (
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (a *API) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		a.logger.Info("HTTP 请求完成", "method", r.Method, "path", r.URL.Path, "status", sw.status, "request_id", r.Header.Get("X-Request-ID"), "duration_ms", time.Since(started).Milliseconds())
	})
}
