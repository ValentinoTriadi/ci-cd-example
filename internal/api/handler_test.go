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

// TestListTodosFilterByDone covers the ?done= query parameter. The store is
// seeded through the API so the test exercises the same path a client does.
func TestListTodosFilterByDone(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, title := range []string{"pending one", "finished", "pending two"} {
		if rec := do(t, srv, http.MethodPost, "/api/todos", `{"title":"`+title+`"}`); rec.Code != http.StatusCreated {
			t.Fatalf("create %q status = %d, want 201 (body %s)", title, rec.Code, rec.Body)
		}
	}
	if rec := do(t, srv, http.MethodPut, "/api/todos/2", `{"title":"finished","done":true}`); rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	tests := []struct {
		name    string
		path    string
		wantIDs []int64
	}{
		{name: "no parameter returns everything", path: "/api/todos", wantIDs: []int64{1, 2, 3}},
		{name: "done=true returns completed", path: "/api/todos?done=true", wantIDs: []int64{2}},
		{name: "done=false returns pending", path: "/api/todos?done=false", wantIDs: []int64{1, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, srv, http.MethodGet, tt.path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200 (body %s)", tt.path, rec.Code, rec.Body)
			}

			got := decodeBody[[]store.Todo](t, rec)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("GET %s returned %d todos, want %d", tt.path, len(got), len(tt.wantIDs))
			}
			for i, todo := range got {
				if todo.ID != tt.wantIDs[i] {
					t.Errorf("todo[%d].ID = %d, want %d", i, todo.ID, tt.wantIDs[i])
				}
			}
		})
	}

	// A filter that matches nothing must still be an array, not null.
	rec := do(t, srv, http.MethodDelete, "/api/todos/2", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	rec = do(t, srv, http.MethodGet, "/api/todos?done=true", "")
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("empty filtered list body = %q, want []", got)
	}
}

// TestListTodosRejectsInvalidDone keeps the parameter strict: only the two
// literals are accepted, so a typo fails loudly instead of silently listing
// everything.
func TestListTodosRejectsInvalidDone(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, path := range []string{"/api/todos?done=maybe", "/api/todos?done=1", "/api/todos?done=TRUE", "/api/todos?done="} {
		rec := do(t, srv, http.MethodGet, path, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want 400 (body %s)", path, rec.Code, rec.Body)
			continue
		}
		if got := decodeBody[map[string]string](t, rec); got["error"] == "" {
			t.Errorf("GET %s body = %s, want an error message", path, rec.Body)
		}
	}
}

// TestStatsEndpoint also pins the routing precedence: /api/todos/stats must
// win over /api/todos/{id} rather than being parsed as an id of "stats".
func TestStatsEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := do(t, srv, http.MethodGet, "/api/todos/stats", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got := decodeBody[store.Stats](t, rec); got != (store.Stats{}) {
		t.Errorf("stats on an empty store = %+v, want zero", got)
	}

	for _, body := range []string{`{"title":"one"}`, `{"title":"two"}`} {
		if seeded := do(t, srv, http.MethodPost, "/api/todos", body); seeded.Code != http.StatusCreated {
			t.Fatalf("seeding failed with status %d", seeded.Code)
		}
	}
	if marked := do(t, srv, http.MethodPut, "/api/todos/1", `{"title":"one","done":true}`); marked.Code != http.StatusOK {
		t.Fatalf("marking done failed with status %d", marked.Code)
	}

	rec = do(t, srv, http.MethodGet, "/api/todos/stats", "")
	want := store.Stats{Total: 2, Done: 1, Pending: 1}
	if got := decodeBody[store.Stats](t, rec); got != want {
		t.Errorf("stats = %+v, want %+v", got, want)
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
