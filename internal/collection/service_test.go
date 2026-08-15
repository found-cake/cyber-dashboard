package collection

import (
	"context"
	"errors"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

func TestServiceKeepsJobRunning_whenRequestContextEnds(t *testing.T) {
	// Given a runner blocked independently from the HTTP request context.
	release := make(chan struct{})
	service := NewService(func(ctx context.Context, day string) (api.CollectionResult, error) {
		<-release
		return api.CollectionResult{Day: day, Collected: 3}, nil
	})
	requestContext, cancelRequest := context.WithCancel(context.Background())

	// When the job starts and its originating request is cancelled.
	job, err := service.Start(requestContext, "2026-08-03")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	cancelRequest()
	close(release)
	completed, err := service.Wait(context.Background(), job.ID)

	// Then server-owned work completes normally.
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if completed.Status != api.CollectionCompleted || completed.Result == nil || completed.Result.Collected != 3 {
		t.Fatalf("job = %+v, want completed result", completed)
	}
}

func TestServiceReturnsActiveJob_whenSameDayStartsAgain(t *testing.T) {
	// Given one active server-owned collection.
	release := make(chan struct{})
	service := NewService(func(context.Context, string) (api.CollectionResult, error) {
		<-release
		return api.CollectionResult{}, nil
	})
	first, err := service.Start(context.Background(), "2026-08-03")
	if err != nil {
		t.Fatalf("start first: %v", err)
	}

	// When the same date is requested again.
	second, err := service.Start(context.Background(), "2026-08-03")

	// Then the existing job is returned instead of launching duplicate work.
	if err != nil {
		t.Fatalf("start second: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("second ID = %q, want %q", second.ID, first.ID)
	}
	close(release)
}

func TestServiceCancelsActiveJob_andReleasesTheSlotForAnotherDay(t *testing.T) {
	// Given a running collection whose runner honours cancellation.
	started := make(chan struct{})
	service := NewService(func(ctx context.Context, day string) (api.CollectionResult, error) {
		close(started)
		<-ctx.Done()
		return api.CollectionResult{Day: day, Collected: 3}, nil
	})
	job, err := service.Start(context.Background(), "2026-08-03")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	<-started

	// When the job is cancelled.
	if err := service.Cancel(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	settled, err := service.Wait(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}

	// Then it settles as cancelled rather than failed, and no job stays active.
	if settled.Status != api.CollectionCancelled {
		t.Fatalf("status = %q, want %q", settled.Status, api.CollectionCancelled)
	}
	if settled.Error != "" {
		t.Fatalf("cancelled job error = %q, want empty", settled.Error)
	}
	if settled.Result != nil {
		t.Fatalf("cancelled job result = %+v, want nil", settled.Result)
	}
	if active := service.Active(); active != nil {
		t.Fatalf("active job = %+v, want none", active)
	}

	// And the freed slot accepts a different day.
	release := make(chan struct{})
	next := NewService(func(context.Context, string) (api.CollectionResult, error) {
		<-release
		return api.CollectionResult{}, nil
	})
	if _, err := next.Start(context.Background(), "2026-08-04"); err != nil {
		t.Fatalf("start next day: %v", err)
	}
	close(release)
}

func TestServiceReportsMissingJob_whenIDIsUnknown(t *testing.T) {
	// Given a service with no jobs.
	service := NewService(func(context.Context, string) (api.CollectionResult, error) {
		return api.CollectionResult{}, nil
	})

	// When an unknown job ID is looked up or cancelled.
	_, getErr := service.Get("collection-404")
	cancelErr := service.Cancel("collection-404")

	// Then both report the stable not-found error callers map to HTTP 404.
	if !errors.Is(getErr, ErrNotFound) {
		t.Fatalf("get error = %v, want ErrNotFound", getErr)
	}
	if !errors.Is(cancelErr, ErrNotFound) {
		t.Fatalf("cancel error = %v, want ErrNotFound", cancelErr)
	}
}

func TestServiceGetReturnsCompletedJob_afterItSettles(t *testing.T) {
	// Given a collection that has finished.
	service := NewService(func(_ context.Context, day string) (api.CollectionResult, error) {
		return api.CollectionResult{Day: day, Collected: 7}, nil
	})
	job, err := service.Start(context.Background(), "2026-08-03")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := service.Wait(context.Background(), job.ID); err != nil {
		t.Fatalf("wait: %v", err)
	}

	// When the job is fetched by ID, as status polling does.
	fetched, err := service.Get(job.ID)

	// Then the settled result is returned.
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Status != api.CollectionCompleted || fetched.Result == nil || fetched.Result.Collected != 7 {
		t.Fatalf("fetched = %+v, want completed result", fetched)
	}
}
