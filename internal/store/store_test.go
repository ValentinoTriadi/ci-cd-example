package store

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCreateAssignsIncrementingIDs(t *testing.T) {
	s := New()

	first, err := s.Create("write the pipeline")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := s.Create("ship it")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if first.ID != 1 || second.ID != 2 {
		t.Errorf("ids = %d, %d; want 1, 2", first.ID, second.ID)
	}
	if s.Len() != 2 {
		t.Errorf("Len() = %d, want 2", s.Len())
	}
}

func TestCreateUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	s := NewWithClock(func() time.Time { return fixed })

	todo, err := s.Create("deterministic")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !todo.CreatedAt.Equal(fixed) {
		t.Errorf("CreatedAt = %v, want %v", todo.CreatedAt, fixed)
	}
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	s := New()

	if _, err := s.Create(""); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("Create(\"\") error = %v, want ErrEmptyTitle", err)
	}
}

func TestListIsSortedByID(t *testing.T) {
	s := New()
	for _, title := range []string{"a", "b", "c"} {
		if _, err := s.Create(title); err != nil {
			t.Fatalf("Create(%q) error = %v", title, err)
		}
	}

	got := s.List()
	if len(got) != 3 {
		t.Fatalf("List() length = %d, want 3", len(got))
	}
	for i, todo := range got {
		if todo.ID != int64(i+1) {
			t.Errorf("List()[%d].ID = %d, want %d", i, todo.ID, i+1)
		}
	}
}

func TestUpdate(t *testing.T) {
	s := New()
	created, err := s.Create("draft")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("replaces title and done flag", func(t *testing.T) {
		updated, err := s.Update(created.ID, "final", true)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.Title != "final" || !updated.Done {
			t.Errorf("Update() = %+v, want title=final done=true", updated)
		}
		if updated.CreatedAt != created.CreatedAt {
			t.Errorf("CreatedAt changed: %v -> %v", created.CreatedAt, updated.CreatedAt)
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		if _, err := s.Update(created.ID, "", false); !errors.Is(err, ErrEmptyTitle) {
			t.Errorf("Update() error = %v, want ErrEmptyTitle", err)
		}
	})

	t.Run("reports missing id", func(t *testing.T) {
		if _, err := s.Update(404, "nope", false); !errors.Is(err, ErrNotFound) {
			t.Errorf("Update() error = %v, want ErrNotFound", err)
		}
	})
}

func TestGetAndDelete(t *testing.T) {
	s := New()
	created, err := s.Create("temporary")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := s.Get(created.ID); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if err := s.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Get(created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
	}
	if err := s.Delete(created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete() twice error = %v, want ErrNotFound", err)
	}
}

// TestConcurrentAccess is the reason CI runs with -race: it hammers the store
// from many goroutines at once. Remove the mutex in store.go and this fails.
func TestConcurrentAccess(t *testing.T) {
	const workers = 32

	s := New()
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if _, err := s.Create("concurrent"); err != nil {
				t.Errorf("Create() error = %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_ = s.List()
			_ = s.Len()
		}()
	}
	wg.Wait()

	if s.Len() != workers {
		t.Errorf("Len() = %d, want %d", s.Len(), workers)
	}
}
