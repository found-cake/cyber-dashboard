package integrationtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
	"github.com/found-cake/cyber-dashboard/internal/vulnerability"
)

func TestRefreshCVEsRunsInBackgroundAndCoalescesDuplicateRequests(t *testing.T) {
	// Given the NVD refresh service remains busy until the test releases it.
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	enricher := &stubVulnerabilityEnricher{
		result:  api.CVERefreshResult{Updated: 4, Removed: 1, Warnings: []string{}},
		started: started,
		release: release,
	}
	server, _, _ := newTestServerWithConfig(t, testServerConfig{
		fetcher: &stubFetcher{}, nvdAPIKey: "test-nvd-key", vulnerabilities: enricher,
	})

	// When the manual refresh endpoint is requested while the refresh is still running.
	firstResponse := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/cves/refresh", nil))
		firstResponse <- recorder
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}

	// Then the endpoint immediately returns one running job instead of holding the connection open.
	var first *httptest.ResponseRecorder
	select {
	case first = <-firstResponse:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("refresh endpoint waited for the long-running NVD job")
	}
	if first.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", first.Code, first.Body.String())
	}
	var firstJob struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstJob); err != nil {
		t.Fatalf("decode first job: %v", err)
	}
	if firstJob.ID == "" || firstJob.Status != "running" {
		t.Fatalf("first job = %+v", firstJob)
	}
	if first.Header().Get("Location") != "/api/cves/refresh/"+firstJob.ID {
		t.Fatalf("location = %q", first.Header().Get("Location"))
	}

	second := httptest.NewRecorder()
	server.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/cves/refresh", nil))
	var secondJob struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondJob); err != nil {
		t.Fatalf("decode second job: %v", err)
	}
	if second.Code != http.StatusAccepted || secondJob.ID != firstJob.ID || enricher.refreshCalls.Load() != 1 {
		t.Fatalf("second status = %d, first = %+v, second = %+v, calls = %d", second.Code, firstJob, secondJob, enricher.refreshCalls.Load())
	}
	bootstrap := httptest.NewRecorder()
	server.ServeHTTP(bootstrap, httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil))
	var bootstrapBody api.Bootstrap
	if err := json.Unmarshal(bootstrap.Body.Bytes(), &bootstrapBody); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if bootstrapBody.CVERefresh == nil || bootstrapBody.CVERefresh.ID != firstJob.ID {
		t.Fatalf("bootstrap refresh = %+v", bootstrapBody.CVERefresh)
	}

	// And polling observes the completed result after the background worker finishes.
	releaseOnce.Do(func() { close(release) })
	completed := httptest.NewRecorder()
	server.ServeHTTP(completed, httptest.NewRequest(http.MethodGet, "/api/cves/refresh/"+firstJob.ID+"?wait=1", nil))
	if completed.Code != http.StatusOK {
		t.Fatalf("poll status = %d, body = %s", completed.Code, completed.Body.String())
	}
	var completedJob struct {
		Status string                `json:"status"`
		Result *api.CVERefreshResult `json:"result"`
	}
	if err := json.Unmarshal(completed.Body.Bytes(), &completedJob); err != nil {
		t.Fatalf("decode completed job: %v", err)
	}
	if completedJob.Status != "completed" || completedJob.Result == nil || completedJob.Result.Updated != 4 || completedJob.Result.Removed != 1 {
		t.Fatalf("completed job = %+v", completedJob)
	}
	bootstrap = httptest.NewRecorder()
	bootstrapBody = api.Bootstrap{}
	server.ServeHTTP(bootstrap, httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil))
	if err := json.Unmarshal(bootstrap.Body.Bytes(), &bootstrapBody); err != nil {
		t.Fatalf("decode terminal bootstrap: %v", err)
	}
	if bootstrapBody.CVERefresh != nil {
		t.Fatalf("terminal refresh remained active: %+v", bootstrapBody.CVERefresh)
	}
	missing := httptest.NewRecorder()
	server.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/cves/refresh/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, body = %s", missing.Code, missing.Body.String())
	}
}

func TestRefreshCVEsReturnsBilingualPrecondition_whenNVDKeyIsMissing(t *testing.T) {
	// Given refresh cannot run because no NVD API key is configured.
	enricher := &stubVulnerabilityEnricher{err: vulnerability.ErrAPIKeyRequired}
	server, _, _ := newTestServerWithConfig(t, testServerConfig{
		fetcher: &stubFetcher{}, vulnerabilities: enricher,
	})

	// When the manual refresh endpoint is requested.
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/cves/refresh", nil))

	// Then both Korean and English guidance are returned to the user.
	if recorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response api.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if response.Code != "nvd_key_required" || response.MessageKO == "" || response.MessageEN == "" || enricher.refreshCalls.Load() != 0 {
		t.Fatalf("response = %+v", response)
	}
}

type stubVulnerabilityEnricher struct {
	result       api.CVERefreshResult
	err          error
	refreshCalls atomic.Int32
	started      chan struct{}
	release      <-chan struct{}
	startOnce    sync.Once
}

func (*stubVulnerabilityEnricher) EnrichDay(context.Context, string) error {
	return nil
}

func (s *stubVulnerabilityEnricher) RefreshAll(context.Context) (api.CVERefreshResult, error) {
	s.refreshCalls.Add(1)
	s.startOnce.Do(func() {
		if s.started != nil {
			close(s.started)
		}
	})
	if s.release != nil {
		<-s.release
	}
	return s.result, s.err
}
