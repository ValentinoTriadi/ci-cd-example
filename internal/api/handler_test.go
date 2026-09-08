package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ValentinoTriadi/ci-cd-example/internal/store"
)

// newTestServer returns a router backed by a fresh store and a silent logger.
func newTestServer(t *testing.T) (srv http.Handler, handler *Handler) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler = NewHandler(store.New(), log)
	srv = handler.Routes()

	return srv, handler
}

func do(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestProbeEndpoints(t *testing.T) {
	srv, h := newTestServer(t)

	tests := []struct {
		name       string
		path       string
		ready      bool
		wantStatus int
		wantField  string
		wantValue  string
	}{
		{"healthz is always ok", "/healthz", true, http.StatusOK, "status", "ok"},
		{"healthz ignores readiness", "/healthz", false, http.StatusOK, "status", "ok"},
		{"readyz when ready", "/readyz", true, http.StatusOK, "status", "ready"},
		{"readyz when draining", "/readyz", false, http.StatusServiceUnavailable, "status", "draining"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.SetReady(tt.ready)

			rec := do(t, srv, http.MethodGet, tt.path, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			body := decodeBody[map[string]string](t, rec)
			if body[tt.wantField] != tt.wantValue {
				t.Errorf("%s = %q, want %q", tt.wantField, body[tt.wantField], tt.wantValue)
			}
		})
	}
}

func TestVersionEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := do(t, srv, http.MethodGet, "/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}

	body := decodeBody[map[string]string](t, rec)
	for _, key := range []string{"version", "commit", "buildDate", "goVersion", "platform"} {
		if body[key] == "" {
			t.Errorf("version payload missing %q: %v", key, body)
		}
	}
}

func TestTodoLifecycle(t *testing.T) {
	srv, _ := newTestServer(t)

	// Empty list first: an unseeded store must return [], not null.
	rec := do(t, srv, http.MethodGet, "/api/todos", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("empty list body = %q, want []", got)
	}

	// Create.
	rec = do(t, srv, http.MethodPost, "/api/todos", `{"title":"wire up CI"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	created := decodeBody[store.Todo](t, rec)
	if created.ID != 1 || created.Title != "wire up CI" || created.Done {
		t.Fatalf("created = %+v", created)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/todos/1" {
		t.Errorf("Location = %q, want /api/todos/1", loc)
	}

	// Read back.
	rec = do(t, srv, http.MethodGet, "/api/todos/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", rec.Code)
	}

	// Update.
	rec = do(t, srv, http.MethodPut, "/api/todos/1", `{"title":"wire up CD","done":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	updated := decodeBody[store.Todo](t, rec)
	if updated.Title != "wire up CD" || !updated.Done {
		t.Errorf("updated = %+v", updated)
	}

	// Delete, then confirm it is gone.
	rec = do(t, srv, http.MethodDelete, "/api/todos/1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	rec = do(t, srv, http.MethodGet, "/api/todos/1", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete status = %d, want 404", rec.Code)
	}
}

func TestRequestValidation(t *testing.T) {
	srv, _ := newTestServer(t)

	if rec := do(t, srv, http.MethodPost, "/api/todos", `{"title":"seed"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seeding failed with status %d", rec.Code)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"blank title", http.MethodPost, "/api/todos", `{"title":"   "}`, http.StatusUnprocessableEntity},
		{"malformed json", http.MethodPost, "/api/todos", `{"title":`, http.StatusBadRequest},
		{"unknown field", http.MethodPost, "/api/todos", `{"titel":"typo"}`, http.StatusBadRequest},
		{"non-numeric id", http.MethodGet, "/api/todos/abc", "", http.StatusBadRequest},
		{"zero id", http.MethodGet, "/api/todos/0", "", http.StatusBadRequest},
		{"missing id", http.MethodGet, "/api/todos/999", "", http.StatusNotFound},
		{"update missing id", http.MethodPut, "/api/todos/999", `{"title":"ghost"}`, http.StatusNotFound},
		{"update blank title", http.MethodPut, "/api/todos/1", `{"title":""}`, http.StatusUnprocessableEntity},
		{"update malformed json", http.MethodPut, "/api/todos/1", `nope`, http.StatusBadRequest},
		{"delete missing id", http.MethodDelete, "/api/todos/999", "", http.StatusNotFound},
		{"delete bad id", http.MethodDelete, "/api/todos/abc", "", http.StatusBadRequest},
		{"method not allowed", http.MethodPatch, "/api/todos/1", "", http.StatusMethodNotAllowed},
		{"unknown route", http.MethodGet, "/nope", "", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, srv, tt.method, tt.path, tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
		})
	}
}
