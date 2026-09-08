package api

import (
	"log/slog"
	"net/http"
	"time"
)

// Routes returns the fully wired mux, middleware included.
//
// Patterns use the method-aware syntax added in Go 1.22, so the mux itself
// handles 405s and path parameters without a third-party router.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /readyz", h.handleReadyz)
	mux.HandleFunc("GET /version", h.handleVersion)

	mux.HandleFunc("GET /api/todos", h.handleListTodos)
	mux.HandleFunc("POST /api/todos", h.handleCreateTodo)
	mux.HandleFunc("GET /api/todos/{id}", h.handleGetTodo)
	mux.HandleFunc("PUT /api/todos/{id}", h.handleUpdateTodo)
	mux.HandleFunc("DELETE /api/todos/{id}", h.handleDeleteTodo)

	return h.withRequestLogging(mux)
}

// statusRecorder captures the status code so the logging middleware can report
// it; http.ResponseWriter alone does not expose what was written.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (h *Handler) withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		h.log.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
