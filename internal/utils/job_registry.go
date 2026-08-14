package utils

import (
	"context"
	"fmt"
	"sync"
)

type activeJobEntry[T any] struct {
	job             T
	done            chan struct{}
	cancel          func()
	cancelRequested bool
	finished        bool
}

// ActiveJobRegistryConfig defines job identity, cloning, and not-found behavior.
type ActiveJobRegistryConfig[T any] struct {
	Prefix   string
	Clone    func(T) T
	WithID   func(T, string) T
	NotFound error
}

// ActiveJobRegistry stores job snapshots while allowing only one active job.
type ActiveJobRegistry[T any] struct {
	mu       sync.Mutex
	prefix   string
	sequence uint64
	clone    func(T) T
	withID   func(T, string) T
	notFound error
	jobs     map[string]*activeJobEntry[T]
	activeID string
}

// NewActiveJobRegistry creates a registry that clones every returned snapshot.
func NewActiveJobRegistry[T any](config ActiveJobRegistryConfig[T]) *ActiveJobRegistry[T] {
	return &ActiveJobRegistry[T]{
		prefix:   config.Prefix,
		clone:    config.Clone,
		withID:   config.WithID,
		notFound: config.NotFound,
		jobs:     make(map[string]*activeJobEntry[T]),
	}
}

// Start stores job when the active slot is free, or returns the current job.
func (r *ActiveJobRegistry[T]) Start(job T, cancel func()) (T, bool) {
	owned := r.clone(job)
	r.mu.Lock()
	if active := r.jobs[r.activeID]; active != nil {
		id, job := r.activeID, active.job
		r.mu.Unlock()
		return r.snapshot(id, job), false
	}
	r.sequence++
	id := fmt.Sprintf("%s-%d", r.prefix, r.sequence)
	r.jobs[id] = &activeJobEntry[T]{job: owned, done: make(chan struct{}), cancel: cancel}
	r.activeID = id
	r.mu.Unlock()
	return r.snapshot(id, owned), true
}

// Active returns a snapshot of the running job.
func (r *ActiveJobRegistry[T]) Active() *T {
	r.mu.Lock()
	entry := r.jobs[r.activeID]
	if entry == nil {
		r.mu.Unlock()
		return nil
	}
	id, job := r.activeID, entry.job
	r.mu.Unlock()
	snapshot := r.snapshot(id, job)
	return &snapshot
}

// Get returns a snapshot of a known job.
func (r *ActiveJobRegistry[T]) Get(id string) (T, error) {
	r.mu.Lock()
	entry := r.jobs[id]
	if entry == nil {
		r.mu.Unlock()
		var zero T
		return zero, r.notFound
	}
	job := entry.job
	r.mu.Unlock()
	return r.snapshot(id, job), nil
}

// Wait returns the completed snapshot or the caller's context error.
func (r *ActiveJobRegistry[T]) Wait(ctx context.Context, id string) (T, error) {
	r.mu.Lock()
	entry := r.jobs[id]
	if entry == nil {
		r.mu.Unlock()
		var zero T
		return zero, r.notFound
	}
	done := entry.done
	r.mu.Unlock()

	select {
	case <-done:
		return r.Get(id)
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// Finish publishes the final snapshot and releases the active slot.
func (r *ActiveJobRegistry[T]) Finish(id string, job T) bool {
	var cancelled T
	return r.finalize(id, r.clone(job), cancelled, false)
}

// FinishCancellable publishes the candidate selected by the first cancellation or finish operation.
func (r *ActiveJobRegistry[T]) FinishCancellable(id string, finished T, cancelled T) bool {
	return r.finalize(id, r.clone(finished), r.clone(cancelled), true)
}

func (r *ActiveJobRegistry[T]) finalize(id string, finished T, cancelled T, cancellable bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.jobs[id]
	if entry == nil || entry.finished {
		return false
	}
	entry.job = finished
	if cancellable && entry.cancelRequested {
		entry.job = cancelled
	}
	entry.cancel = nil
	entry.finished = true
	if r.activeID == id {
		r.activeID = ""
	}
	close(entry.done)
	return true
}

// Cancel invokes a known active job's cancellation callback at most once.
func (r *ActiveJobRegistry[T]) Cancel(id string) error {
	r.mu.Lock()
	entry := r.jobs[id]
	if entry == nil {
		r.mu.Unlock()
		return r.notFound
	}
	var cancel func()
	if !entry.finished {
		entry.cancelRequested = true
		cancel = entry.cancel
		entry.cancel = nil
	}
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (r *ActiveJobRegistry[T]) snapshot(id string, job T) T {
	return r.withID(r.clone(job), id)
}
