package main

import (
	"log/slog"
	"net/http"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/storage/jsonstore"
	"confinedpermit/internal/transport/httpapi"
)

type runtime struct {
	store   *jsonstore.Store
	service *application.Service
	handler http.Handler
}

func buildRuntime(dataPath string, logger *slog.Logger) (*runtime, error) {
	store, err := jsonstore.Open(dataPath)
	if err != nil {
		return nil, err
	}
	service := application.New(store)
	return &runtime{store: store, service: service, handler: httpapi.New(service, logger)}, nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
