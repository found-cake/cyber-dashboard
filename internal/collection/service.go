package collection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/found-cake/cyber-dashboard/api"
)

var ErrBusy = errors.New("another collection is already running")
var ErrNotFound = errors.New("collection job not found")

type Runner func(context.Context, string) (api.CollectionResult, error)

type jobState struct {
	job    api.CollectionJob
	done   chan struct{}
	cancel context.CancelFunc
}

type Service struct {
	mu       sync.Mutex
	sequence atomic.Uint64
	runner   Runner
	jobs     map[string]*jobState
	activeID string
}

func NewService(runner Runner) *Service {
	return &Service{runner: runner, jobs: make(map[string]*jobState)}
}

func (s *Service) Start(_ context.Context, day string) (api.CollectionJob, error) {
	s.mu.Lock()
	if active := s.jobs[s.activeID]; active != nil {
		job := cloneJob(active.job)
		s.mu.Unlock()
		if job.Day == day {
			return job, nil
		}
		return api.CollectionJob{}, ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	id := fmt.Sprintf("collection-%d", s.sequence.Add(1))
	state := &jobState{job: api.CollectionJob{ID: id, Day: day, Status: api.CollectionRunning}, done: make(chan struct{}), cancel: cancel}
	s.jobs[id] = state
	s.activeID = id
	job := cloneJob(state.job)
	s.mu.Unlock()
	go s.run(ctx, state)
	return job, nil
}

func (s *Service) run(ctx context.Context, state *jobState) {
	result, err := s.runner(ctx, state.job.Day)
	s.mu.Lock()
	defer s.mu.Unlock()
	if errors.Is(ctx.Err(), context.Canceled) {
		state.job.Status = api.CollectionCancelled
	} else if err != nil {
		state.job.Status = api.CollectionFailed
		state.job.Error = "수집에 실패했습니다 / Collection failed"
	} else {
		state.job.Status = api.CollectionCompleted
		state.job.Result = &result
	}
	if s.activeID == state.job.ID {
		s.activeID = ""
	}
	close(state.done)
}

func (s *Service) Active() *api.CollectionJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.jobs[s.activeID]
	if state == nil {
		return nil
	}
	job := cloneJob(state.job)
	return &job
}

func (s *Service) Get(id string) (api.CollectionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.jobs[id]
	if state == nil {
		return api.CollectionJob{}, ErrNotFound
	}
	return cloneJob(state.job), nil
}

func (s *Service) Wait(ctx context.Context, id string) (api.CollectionJob, error) {
	s.mu.Lock()
	state := s.jobs[id]
	if state == nil {
		s.mu.Unlock()
		return api.CollectionJob{}, ErrNotFound
	}
	done := state.done
	s.mu.Unlock()
	select {
	case <-done:
	case <-ctx.Done():
		return api.CollectionJob{}, ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneJob(state.job), nil
}

func (s *Service) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.jobs[id]
	if state == nil {
		return ErrNotFound
	}
	if state.job.Status == api.CollectionRunning {
		state.cancel()
	}
	return nil
}

func cloneJob(job api.CollectionJob) api.CollectionJob {
	if job.Result != nil {
		result := *job.Result
		result.Warnings = append([]string(nil), result.Warnings...)
		job.Result = &result
	}
	return job
}
