package collection

import (
	"context"
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
