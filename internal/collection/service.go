package collection

import (
	"context"
	"errors"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/utils"
)

var ErrBusy = errors.New("another collection is already running")
var ErrNotFound = errors.New("collection job not found")

type Runner func(context.Context, string) (api.CollectionResult, error)

type Service struct {
	runner Runner
	jobs   *utils.ActiveJobRegistry[api.CollectionJob]
}

func NewService(runner Runner) *Service {
	return &Service{
		runner: runner,
		jobs: utils.NewActiveJobRegistry(utils.ActiveJobRegistryConfig[api.CollectionJob]{
			Prefix: "collection", Clone: cloneJob, WithID: withCollectionJobID, NotFound: ErrNotFound,
		}),
	}
}

func (s *Service) Start(_ context.Context, day string) (api.CollectionJob, error) {
	ctx, cancel := context.WithCancel(context.Background())
	job, started := s.jobs.Start(api.CollectionJob{Day: day, Status: api.CollectionRunning}, cancel)
	if !started {
		cancel()
		if job.Day == day {
			return job, nil
		}
		return api.CollectionJob{}, ErrBusy
	}
	go s.run(ctx, job)
	return job, nil
}

func (s *Service) run(ctx context.Context, job api.CollectionJob) {
	result, err := s.runner(ctx, job.Day)
	finished := job
	if err != nil {
		finished.Status = api.CollectionFailed
		finished.Error = "수집에 실패했습니다 / Collection failed"
	} else {
		finished.Status = api.CollectionCompleted
		finished.Result = &result
	}
	cancelled := job
	cancelled.Status = api.CollectionCancelled
	cancelled.Result = nil
	cancelled.Error = ""
	s.jobs.FinishCancellable(job.ID, finished, cancelled)
}

func (s *Service) Active() *api.CollectionJob {
	return s.jobs.Active()
}

func (s *Service) Get(id string) (api.CollectionJob, error) {
	return s.jobs.Get(id)
}

func (s *Service) Wait(ctx context.Context, id string) (api.CollectionJob, error) {
	return s.jobs.Wait(ctx, id)
}

func (s *Service) Cancel(id string) error {
	return s.jobs.Cancel(id)
}

func cloneJob(job api.CollectionJob) api.CollectionJob {
	if job.Result != nil {
		result := *job.Result
		result.Warnings = append([]string(nil), result.Warnings...)
		job.Result = &result
	}
	return job
}

func withCollectionJobID(job api.CollectionJob, id string) api.CollectionJob {
	job.ID = id
	return job
}
