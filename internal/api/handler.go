// Package api wires the HTTP surface of the demo service.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ValentinoTriadi/ci-cd-example/internal/store"
	"github.com/ValentinoTriadi/ci-cd-example/internal/version"
)

// Handler serves the API. Build one with NewHandler and mount its Routes.
type Handler struct {
	store *store.Store
	log   *slog.Logger
	ready atomic.Bool
}

// NewHandler returns a Handler that starts out ready to serve traffic.
func NewHandler(s *store.Store, log *slog.Logger) *Handler {
	h := &Handler{store: s, log: log}
	h.ready.Store(true)
	return h
}

// SetReady flips the readiness flag. main flips it to false as soon as a
// shutdown signal arrives, so load balancers drain the instance before the
// listener actually closes.
func (h *Handler) SetReady(ready bool) { h.ready.Store(ready) }

type errorResponse struct {
	Error string `json:"error"`
}

type createTodoRequest struct {
	Title string `json:"title"`
}

type updateTodoRequest struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.log.Error("failed to encode response", slog.Any("error", err))
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, errorResponse{Error: msg})
}

// handleHealthz reports process liveness. It stays 200 until the process dies.
func (h *Handler) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz reports whether the process should receive traffic.
func (h *Handler) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		h.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleVersion returns the build metadata injected at link time. The release
// workflow asserts that this matches the git tag it built from.
func (h *Handler) handleVersion(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, version.Get())
}

func (h *Handler) handleStats(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, h.store.Snapshot())
}

func (h *Handler) handleListTodos(w http.ResponseWriter, r *http.Request) {
	done, err := doneFilter(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.store.Filter(done))
}

func (h *Handler) handleCreateTodo(w http.ResponseWriter, r *http.Request) {
	var req createTodoRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	todo, err := h.store.Create(strings.TrimSpace(req.Title))
	if err != nil {
		h.writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Location", "/api/todos/"+strconv.FormatInt(todo.ID, 10))
	h.writeJSON(w, http.StatusCreated, todo)
}

func (h *Handler) handleGetTodo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	todo, err := h.store.Get(id)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, todo)
}

func (h *Handler) handleUpdateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req updateTodoRequest
	if decodeErr := decodeJSON(r, &req); decodeErr != nil {
		h.writeError(w, http.StatusBadRequest, decodeErr.Error())
		return
	}

	todo, err := h.store.Update(id, strings.TrimSpace(req.Title), req.Done)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, todo)
}

func (h *Handler) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.Delete(id); err != nil {
		h.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeStoreError maps store errors onto status codes in one place, so every
// handler reports the same thing for the same failure.
func (h *Handler) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		h.writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrEmptyTitle):
		h.writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		h.log.Error("unexpected store error", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return errors.New("invalid JSON body: " + err.Error())
	}
	return nil
}

// doneFilter reads the optional ?done= parameter, returning nil when it is
// absent. Only "true" and "false" are accepted; ParseBool would also take "1",
// "t" and "TRUE", a wider surface than the endpoint documents.
func doneFilter(r *http.Request) (*bool, error) {
	query := r.URL.Query()
	if !query.Has("done") {
		return nil, nil
	}

	switch query.Get("done") {
	case "true":
		done := true
		return &done, nil
	case "false":
		done := false
		return &done, nil
	default:
		return nil, errors.New(`done must be "true" or "false"`)
	}
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("id must be a positive integer")
	}
	return id, nil
}
