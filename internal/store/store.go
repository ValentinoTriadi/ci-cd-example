// Package store holds the in-memory todo data used by the demo API.
//
// It is deliberately concurrency-safe: the pipeline runs `go test -race`, and
// this is the package that makes that flag earn its keep.
package store

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrNotFound is returned when no todo exists for the requested ID.
var ErrNotFound = errors.New("todo not found")

// ErrEmptyTitle is returned when a todo is created or updated without a title.
var ErrEmptyTitle = errors.New("title must not be empty")

// Todo is a single item of work.
type Todo struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store is a goroutine-safe in-memory collection of todos.
type Store struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64]Todo
	now    func() time.Time
}

// New returns an empty Store backed by the real clock.
func New() *Store {
	return &Store{
		nextID: 1,
		items:  make(map[int64]Todo),
		now:    time.Now,
	}
}

// NewWithClock returns an empty Store using the supplied clock, so tests can
// assert on CreatedAt without sleeping.
func NewWithClock(now func() time.Time) *Store {
	s := New()
	s.now = now
	return s
}

// Create adds a todo and returns the stored copy, including its assigned ID.
func (s *Store) Create(title string) (Todo, error) {
	if title == "" {
		return Todo{}, ErrEmptyTitle
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	todo := Todo{
		ID:        s.nextID,
		Title:     title,
		CreatedAt: s.now().UTC(),
	}
	s.items[todo.ID] = todo
	s.nextID++

	return todo, nil
}

// Get returns the todo with the given ID.
func (s *Store) Get(id int64) (Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	todo, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	return todo, nil
}

// List returns every todo ordered by ID, so responses are deterministic.
func (s *Store) List() []Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Todo, 0, len(s.items))
	for _, todo := range s.items {
		out = append(out, todo)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	return out
}

// Update replaces the title and done flag of an existing todo.
func (s *Store) Update(id int64, title string, done bool) (Todo, error) {
	if title == "" {
		return Todo{}, ErrEmptyTitle
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	todo, ok := s.items[id]
	if !ok {
		return Todo{}, ErrNotFound
	}

	todo.Title = title
	todo.Done = done
	s.items[id] = todo

	return todo, nil
}

// Delete removes a todo by ID.
func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)

	return nil
}

// Stats is an aggregate view of the collection.
type Stats struct {
	Total   int `json:"total"`
	Done    int `json:"done"`
	Pending int `json:"pending"`
}

// Snapshot returns the current counts in a single lock acquisition, so the
// three numbers are always consistent with each other.
func (s *Store) Snapshot() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := Stats{Total: len(s.items)}
	for _, todo := range s.items {
		if todo.Done {
			stats.Done++
		}
	}
	stats.Pending = stats.Total - stats.Done

	return stats
}

// Len reports how many todos are stored.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items)
}
