package utils

import (
	"context"
	"errors"
	"testing"
)

type registryJob struct {
	ID     string
	Status string
	Values []string
}

var errRegistryJobNotFound = errors.New("registry job not found")

func cloneRegistryJob(job registryJob) registryJob {
	job.Values = append([]string(nil), job.Values...)
	return job
}

func newTestJobRegistry() *ActiveJobRegistry[registryJob] {
	return NewActiveJobRegistry(ActiveJobRegistryConfig[registryJob]{
		Prefix:   "job",
		Clone:    cloneRegistryJob,
		WithID:   func(job registryJob, id string) registryJob { job.ID = id; return job },
		NotFound: errRegistryJobNotFound,
	})
}

func TestActiveJobRegistryReturnsExistingJob_whenStartOverlaps(t *testing.T) {
	// Given an idle single-active registry.
	registry := newTestJobRegistry()

	// When two jobs are started without finishing the first.
	first, firstStarted := registry.Start(registryJob{Status: "running"}, nil)
	second, secondStarted := registry.Start(registryJob{Status: "running"}, nil)

	// Then only the first job becomes active.
	if !firstStarted || first.ID != "job-1" {
		t.Fatalf("first start = (%+v, %t), want job-1 started", first, firstStarted)
	}
	if secondStarted || second.ID != first.ID {
		t.Fatalf("second start = (%+v, %t), want existing job-1", second, secondStarted)
	}
	active := registry.Active()
	if active == nil || active.ID != first.ID {
		t.Fatalf("active = %+v, want job-1", active)
	}
	registry.Finish(first.ID, registryJob{Status: "completed"})
	next, nextStarted := registry.Start(registryJob{Status: "running"}, nil)
	if !nextStarted || next.ID != "job-2" {
		t.Fatalf("next start = (%+v, %t), want job-2 started", next, nextStarted)
	}
}

func TestActiveJobRegistryPublishesClonedSnapshot_whenJobFinishes(t *testing.T) {
	// Given one active job.
	registry := newTestJobRegistry()
	startedJob, started := registry.Start(registryJob{Status: "running"}, nil)
	if !started {
		t.Fatal("start job-1: registry was unexpectedly busy")
	}

	// When the job finishes and a waiter mutates its returned snapshot.
	finished := registryJob{Status: "completed", Values: []string{"stored"}}
	if !registry.Finish(startedJob.ID, finished) {
		t.Fatal("finish job-1: job was not found")
	}
	waited, err := registry.Wait(context.Background(), startedJob.ID)
	if err != nil {
		t.Fatalf("wait = (%+v, %v), want completed job", waited, err)
	}
	waited.Values[0] = "mutated"

	// Then stored state stays isolated and the active slot is released.
	stored, err := registry.Get(startedJob.ID)
	if err != nil || stored.Status != "completed" || stored.Values[0] != "stored" {
		t.Fatalf("stored = (%+v, %v), want immutable completed snapshot", stored, err)
	}
	if active := registry.Active(); active != nil {
		t.Fatalf("active = %+v, want no active job", active)
	}
}

func TestActiveJobRegistryWaitReturnsContextError_whenCallerStopsWaiting(t *testing.T) {
	// Given an active job and a cancelled waiter context.
	registry := newTestJobRegistry()
	started, _ := registry.Start(registryJob{Status: "running"}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When the caller stops waiting.
	_, err := registry.Wait(ctx, started.ID)

	// Then the job is still found and the context error is returned.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context cancellation", err)
	}
}

func TestActiveJobRegistryInvokesCancelOnce_whenCancellationRepeats(t *testing.T) {
	// Given an active job with a cancellation callback.
	registry := newTestJobRegistry()
	calls := 0
	callbackUnderLock := false
	started, _ := registry.Start(registryJob{Status: "running"}, func() {
		if registry.mu.TryLock() {
			registry.mu.Unlock()
		} else {
			callbackUnderLock = true
		}
		calls++
	})

	// When cancellation is requested repeatedly.
	firstErr := registry.Cancel(started.ID)
	secondErr := registry.Cancel(started.ID)

	// Then the known job is reported both times but its callback runs once.
	if firstErr != nil || secondErr != nil || calls != 1 || callbackUnderLock {
		t.Fatalf("cancel = (%v, %v), calls = %d, callback under lock = %t; want known job and one unlocked callback", firstErr, secondErr, calls, callbackUnderLock)
	}
	if err := registry.Cancel("missing"); !errors.Is(err, errRegistryJobNotFound) {
		t.Fatalf("cancel missing job = %v, want not found", err)
	}
}

func TestActiveJobRegistryClonesFinalCandidates_outsideLock(t *testing.T) {
	// Given a registry whose clone operation observes mutex ownership.
	var registry *ActiveJobRegistry[registryJob]
	checkLock := false
	cloneCalls := 0
	cloneUnderLock := false
	registry = NewActiveJobRegistry(ActiveJobRegistryConfig[registryJob]{
		Prefix: "job",
		Clone: func(job registryJob) registryJob {
			if checkLock {
				cloneCalls++
				if registry.mu.TryLock() {
					registry.mu.Unlock()
				} else {
					cloneUnderLock = true
				}
			}
			return cloneRegistryJob(job)
		},
		WithID:   func(job registryJob, id string) registryJob { job.ID = id; return job },
		NotFound: errRegistryJobNotFound,
	})
	started, _ := registry.Start(registryJob{Status: "running"}, nil)
	checkLock = true

	// When both final candidates are prepared.
	finished := registry.FinishCancellable(
		started.ID,
		registryJob{Status: "completed"},
		registryJob{Status: "cancelled"},
	)

	// Then each clone runs before finalization acquires the mutex.
	if !finished || cloneCalls != 2 || cloneUnderLock {
		t.Fatalf("finish = %t, clone calls = %d, clone under lock = %t; want two unlocked clones", finished, cloneCalls, cloneUnderLock)
	}
}

func TestActiveJobRegistryPublishesCancelledCandidate_whenCancelWins(t *testing.T) {
	// Given a cancellation callback blocked after Cancel has released the registry mutex.
	registry := newTestJobRegistry()
	cancelEntered := make(chan struct{})
	releaseCancel := make(chan struct{})
	started, _ := registry.Start(registryJob{Status: "running"}, func() {
		close(cancelEntered)
		<-releaseCancel
	})
	cancelDone := make(chan error, 1)
	go func() {
		cancelDone <- registry.Cancel(started.ID)
	}()
	<-cancelEntered

	// When completion races after cancellation has already linearized.
	finished := registry.FinishCancellable(
		started.ID,
		registryJob{Status: "completed"},
		registryJob{Status: "cancelled"},
	)
	close(releaseCancel)
	if err := <-cancelDone; err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// Then cancellation wins and publishes the cancelled terminal candidate.
	settled, err := registry.Get(started.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !finished || settled.Status != "cancelled" {
		t.Fatalf("finish = %t, status = %q; want cancelled", finished, settled.Status)
	}
}

func TestActiveJobRegistryPublishesFinishedCandidate_whenFinishWins(t *testing.T) {
	// Given an active job with an observable cancellation callback.
	registry := newTestJobRegistry()
	cancelCalled := make(chan struct{}, 1)
	started, _ := registry.Start(registryJob{Status: "running"}, func() {
		cancelCalled <- struct{}{}
	})
	finishDone := make(chan bool, 1)

	// When finish closes the job barrier before cancellation is requested.
	go func() {
		finishDone <- registry.FinishCancellable(
			started.ID,
			registryJob{Status: "completed"},
			registryJob{Status: "cancelled"},
		)
	}()
	settled, err := registry.Wait(context.Background(), started.ID)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if err := registry.Cancel(started.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// Then completion remains published and the callback is not invoked.
	if finished := <-finishDone; !finished {
		t.Fatal("finish = false, want true")
	}
	if settled.Status != "completed" {
		t.Fatalf("status = %q, want completed", settled.Status)
	}
	select {
	case <-cancelCalled:
		t.Fatal("cancel callback invoked after finish won")
	default:
	}
}
