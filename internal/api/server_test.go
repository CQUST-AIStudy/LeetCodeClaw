package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"leetcodeclaw/internal/leetcode"
	"leetcodeclaw/internal/storage"
)

func TestHealthWithoutStore(t *testing.T) {
	server := NewServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["success"] != true {
		t.Fatalf("success = %v", body["success"])
	}
	if body["database"] != "disabled" {
		t.Fatalf("database = %v", body["database"])
	}
}

func TestTypedNilStoreIsTreatedAsDisabled(t *testing.T) {
	var store *storage.Store
	server := NewServer(nil, store)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["database"] != "disabled" {
		t.Fatalf("database = %v, want disabled", body["database"])
	}
}

func TestReadyWithoutStoreReturnsServiceUnavailable(t *testing.T) {
	server := NewServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["success"] != false || body["database"] != "disabled" {
		t.Fatalf("body = %#v", body)
	}
}

func TestReadyWithHealthyStoreReturnsOK(t *testing.T) {
	store := newFakeCrawlJobStore()
	server := NewServer(nil, store)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["success"] != true || body["database"] != "ok" || body["schema"] != "ok" {
		t.Fatalf("body = %#v", body)
	}
}

func TestReadyWithSchemaErrorReturnsServiceUnavailable(t *testing.T) {
	store := newFakeCrawlJobStore()
	store.schemaErr = errors.New("missing table")
	server := NewServer(nil, store)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["success"] != false || body["database"] != "ok" || body["schema"] != "missing table" {
		t.Fatalf("body = %#v", body)
	}
}

func TestAPIKeyRejectsMissingKey(t *testing.T) {
	server := NewServerWithConfig(&fakeProblemService{}, nil, ServerConfig{APIKey: "secret"})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/leetcode/recommend/keyword",
		strings.NewReader(`{"keyword":"binary","persist":false}`),
	)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestAPIKeyAcceptsBearerToken(t *testing.T) {
	server := NewServerWithConfig(&fakeProblemService{}, nil, ServerConfig{APIKey: "secret"})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/leetcode/recommend/keyword",
		strings.NewReader(`{"keyword":"binary","persist":false}`),
	)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestAPIKeyAcceptsHeaderToken(t *testing.T) {
	server := NewServerWithConfig(&fakeProblemService{}, nil, ServerConfig{APIKey: "secret"})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/leetcode/recommend/keyword",
		strings.NewReader(`{"keyword":"binary","persist":false}`),
	)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestAPIKeySkipsHealthAndOptions(t *testing.T) {
	server := NewServerWithConfig(&fakeProblemService{}, nil, ServerConfig{APIKey: "secret"})
	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()

	server.Routes().ServeHTTP(healthRec, healthReq)

	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	optionsReq := httptest.NewRequest(http.MethodOptions, "/api/leetcode/crawl", nil)
	optionsRec := httptest.NewRecorder()

	server.Routes().ServeHTTP(optionsRec, optionsReq)

	if optionsRec.Code != http.StatusNoContent {
		t.Fatalf("options status = %d, want %d", optionsRec.Code, http.StatusNoContent)
	}
}

func TestCORSDefaultsToAllowAll(t *testing.T) {
	server := NewServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	server := NewServerWithConfig(nil, nil, ServerConfig{CORSOrigins: []string{"https://app.example.com"}})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	server := NewServerWithConfig(nil, nil, ServerConfig{CORSOrigins: []string{"https://app.example.com"}})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCrawlAllWithoutStoreReturnsServiceUnavailable(t *testing.T) {
	var store *storage.Store
	server := NewServer(&fakeProblemService{}, store)
	req := httptest.NewRequest(http.MethodPost, "/api/leetcode/crawl/all", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "database store is required") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestNormalizeSlugsDeduplicates(t *testing.T) {
	got := normalizeSlugs([]string{"two-sum", " ", "Two-Sum", "add-two-numbers"})
	want := []string{"two-sum", "add-two-numbers"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandedRecommendLimitCapsAtFifty(t *testing.T) {
	if got := expandedRecommendLimit(20); got != 50 {
		t.Fatalf("expandedRecommendLimit(20) = %d, want 50", got)
	}
	if got := expandedRecommendLimit(2); got != 6 {
		t.Fatalf("expandedRecommendLimit(2) = %d, want 6", got)
	}
}

func TestKeywordRecommendOmitsMissingSolutionContent(t *testing.T) {
	service := &fakeProblemService{
		candidates: []leetcode.SearchCandidate{
			{TitleSlug: "missing-solution", Title: "Missing Solution", TranslatedTitle: "缺失题解", Score: 0.95, Reason: "matched"},
			{TitleSlug: "valid-solution", Title: "Valid Solution", TranslatedTitle: "可用题解", Score: 0.90, Reason: "matched"},
		},
		problems: map[string]leetcode.Problem{
			"missing-solution": {
				Title:           "Missing Solution",
				TitleSlug:       "missing-solution",
				TranslatedTitle: "缺失题解",
				Solution: leetcode.Solution{
					SourceSlug:     "missing-solution",
					CodeByLanguage: map[string]string{},
				},
			},
			"valid-solution": {
				Title:           "Valid Solution",
				TitleSlug:       "valid-solution",
				TranslatedTitle: "可用题解",
				Solution: leetcode.Solution{
					SourceSlug:       "valid-solution",
					ContentMarkdown:  "solution content",
					CodeByLanguage:   map[string]string{"c": "int main() { return 0; }"},
					MissingLanguages: map[string]string{},
				},
			},
		},
	}
	server := NewServer(service, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/leetcode/recommend/keyword",
		strings.NewReader(`{"keyword":"binary","limit":2,"persist":false}`),
	)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.searchOptions.Limit != 6 {
		t.Fatalf("search limit = %d, want 6", service.searchOptions.Limit)
	}
	var body keywordRecommendResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items len = %d, want 1: %+v", len(body.Items), body.Items)
	}
	if body.Items[0].Problem.TitleSlug != "valid-solution" {
		t.Fatalf("item slug = %q, want valid-solution", body.Items[0].Problem.TitleSlug)
	}
	if len(body.Omitted) != 1 {
		t.Fatalf("omitted len = %d, want 1: %+v", len(body.Omitted), body.Omitted)
	}
	if body.Omitted[0].Slug != "missing-solution" {
		t.Fatalf("omitted slug = %q, want missing-solution", body.Omitted[0].Slug)
	}
	if body.Omitted[0].Reason != "题解正文缺失" {
		t.Fatalf("omitted reason = %q", body.Omitted[0].Reason)
	}
	if len(body.Omitted[0].Errors) == 0 {
		t.Fatal("expected omitted item to include validation errors")
	}
}

func TestCrawlAllStartsBackgroundJobAndRecordsResult(t *testing.T) {
	service := &fakeProblemService{
		publicProblems: []leetcode.SearchCandidate{
			{TitleSlug: "valid-solution"},
			{TitleSlug: "failed-solution"},
		},
		problems: map[string]leetcode.Problem{
			"valid-solution": {
				Title:           "Valid Solution",
				TitleSlug:       "valid-solution",
				TranslatedTitle: "有效题解",
				Solution: leetcode.Solution{
					SourceSlug:       "valid-solution",
					ContentMarkdown:  "solution content",
					CodeByLanguage:   map[string]string{"c": "int main() { return 0; }"},
					MissingLanguages: map[string]string{},
				},
			},
		},
	}
	store := newFakeCrawlJobStore()
	server := NewServerWithCrawlAllConfig(service, store, CrawlAllConfig{Workers: 1, PageSize: 2})
	req := httptest.NewRequest(http.MethodPost, "/api/leetcode/crawl/all", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var started crawlAllResponse
	if err := json.NewDecoder(rec.Body).Decode(&started); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if started.JobID == 0 {
		t.Fatal("expected job id")
	}

	detail := waitForCrawlJobStatus(t, store, started.JobID, storage.CrawlJobStatusPartialFailed)
	if detail.Job.Total != 2 || detail.Job.Succeeded != 1 || detail.Job.Failed != 1 {
		t.Fatalf("job counts = total:%d succeeded:%d failed:%d", detail.Job.Total, detail.Job.Succeeded, detail.Job.Failed)
	}
	if len(detail.FailedItems) != 1 || detail.FailedItems[0].Slug != "failed-solution" {
		t.Fatalf("failed items = %+v", detail.FailedItems)
	}
	if service.publicOptions.PageSize != 2 {
		t.Fatalf("page size = %d, want 2", service.publicOptions.PageSize)
	}
}

func TestCrawlAllRejectsDuplicateActiveJob(t *testing.T) {
	store := newFakeCrawlJobStore()
	store.jobs[1] = storage.CrawlJob{
		ID:        1,
		Status:    storage.CrawlJobStatusRunning,
		Persist:   true,
		Workers:   1,
		PageSize:  100,
		CreatedAt: time.Now(),
	}
	store.nextJobID = 2
	server := NewServer(&fakeProblemService{}, store)
	req := httptest.NewRequest(http.MethodPost, "/api/leetcode/crawl/all", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	var body crawlAllResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.JobID != 1 || body.Status != storage.CrawlJobStatusRunning {
		t.Fatalf("duplicate response = %+v", body)
	}
}

type fakeProblemService struct {
	candidates     []leetcode.SearchCandidate
	publicProblems []leetcode.SearchCandidate
	problems       map[string]leetcode.Problem
	searchOptions  leetcode.SearchOptions
	publicOptions  leetcode.PublicProblemListOptions
}

func (s *fakeProblemService) SearchProblems(_ context.Context, options leetcode.SearchOptions) ([]leetcode.SearchCandidate, error) {
	s.searchOptions = options
	return s.candidates, nil
}

func (s *fakeProblemService) ListPublicProblems(_ context.Context, options leetcode.PublicProblemListOptions) ([]leetcode.SearchCandidate, error) {
	s.publicOptions = options
	return s.publicProblems, nil
}

func (s *fakeProblemService) CrawlProblem(_ context.Context, slug string) (leetcode.Problem, error) {
	problem, ok := s.problems[slug]
	if !ok {
		return leetcode.Problem{}, errors.New("not found")
	}
	return problem, nil
}

type fakeCrawlJobStore struct {
	mu            sync.Mutex
	jobs          map[int64]storage.CrawlJob
	items         map[int64][]storage.CrawlJobItem
	nextJobID     int64
	nextItemID    int64
	nextProblemID int64
	pingErr       error
	schemaErr     error
}

func newFakeCrawlJobStore() *fakeCrawlJobStore {
	return &fakeCrawlJobStore{
		jobs:          map[int64]storage.CrawlJob{},
		items:         map[int64][]storage.CrawlJobItem{},
		nextJobID:     1,
		nextItemID:    1,
		nextProblemID: 1,
	}
}

func (s *fakeCrawlJobStore) Ping(context.Context) error {
	return s.pingErr
}

func (s *fakeCrawlJobStore) CheckSchema(context.Context) error {
	return s.schemaErr
}

func (s *fakeCrawlJobStore) UpsertProblem(_ context.Context, problem leetcode.Problem) (storage.PersistResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(problem.Solution.ContentMarkdown) == "" {
		return storage.PersistResult{}, errors.New("solution content is empty")
	}
	id := s.nextProblemID
	s.nextProblemID++
	return storage.PersistResult{ProblemID: id, Inserted: true}, nil
}

func (s *fakeCrawlJobStore) FindProblemBySlug(context.Context, string) (*leetcode.Problem, error) {
	return nil, nil
}

func (s *fakeCrawlJobStore) FindActiveCrawlJob(context.Context) (*storage.CrawlJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.Status == storage.CrawlJobStatusQueued || job.Status == storage.CrawlJobStatusRunning {
			copy := job
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *fakeCrawlJobStore) CreateCrawlJob(_ context.Context, cfg storage.CrawlJobConfig) (storage.CrawlJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextJobID
	s.nextJobID++
	job := storage.CrawlJob{
		ID:           id,
		Status:       storage.CrawlJobStatusQueued,
		Persist:      cfg.Persist,
		ForceRefresh: cfg.ForceRefresh,
		Workers:      cfg.Workers,
		Delay:        cfg.Delay,
		PageSize:     cfg.PageSize,
		CreatedAt:    time.Now(),
	}
	s.jobs[id] = job
	return job, nil
}

func (s *fakeCrawlJobStore) AddCrawlJobItems(_ context.Context, jobID int64, slugs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, slug := range slugs {
		item := storage.CrawlJobItem{
			ID:        s.nextItemID,
			JobID:     jobID,
			Slug:      slug,
			Status:    storage.CrawlJobItemStatusPending,
			CreatedAt: time.Now(),
		}
		s.nextItemID++
		s.items[jobID] = append(s.items[jobID], item)
	}
	return nil
}

func (s *fakeCrawlJobStore) MarkCrawlJobRunning(_ context.Context, jobID int64, total int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[jobID]
	now := time.Now()
	job.Status = storage.CrawlJobStatusRunning
	job.Total = total
	job.StartedAt = &now
	s.jobs[jobID] = job
	return nil
}

func (s *fakeCrawlJobStore) ListCrawlJobItemsByStatus(_ context.Context, jobID int64, status string) ([]storage.CrawlJobItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []storage.CrawlJobItem{}
	for _, item := range s.items[jobID] {
		if item.Status == status {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *fakeCrawlJobStore) MarkCrawlJobItemRunning(_ context.Context, itemID int64) error {
	return s.updateItem(itemID, func(item *storage.CrawlJobItem) {
		now := time.Now()
		item.Status = storage.CrawlJobItemStatusRunning
		item.Attempts++
		item.StartedAt = &now
		item.Error = ""
	})
}

func (s *fakeCrawlJobStore) MarkCrawlJobItemSucceeded(_ context.Context, itemID int64, problemID int64) error {
	return s.updateItem(itemID, func(item *storage.CrawlJobItem) {
		now := time.Now()
		item.Status = storage.CrawlJobItemStatusSucceeded
		item.ProblemID = problemID
		item.FinishedAt = &now
		item.Error = ""
	})
}

func (s *fakeCrawlJobStore) MarkCrawlJobItemFailed(_ context.Context, itemID int64, message string) error {
	return s.updateItem(itemID, func(item *storage.CrawlJobItem) {
		now := time.Now()
		item.Status = storage.CrawlJobItemStatusFailed
		item.Error = message
		item.FinishedAt = &now
	})
}

func (s *fakeCrawlJobStore) FinishCrawlJob(_ context.Context, jobID int64) (storage.CrawlJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[jobID]
	job.Total = len(s.items[jobID])
	job.Succeeded = 0
	job.Failed = 0
	for _, item := range s.items[jobID] {
		switch item.Status {
		case storage.CrawlJobItemStatusSucceeded:
			job.Succeeded++
		case storage.CrawlJobItemStatusFailed:
			job.Failed++
		default:
			job.Failed++
		}
	}
	switch {
	case job.Failed > 0 && job.Succeeded > 0:
		job.Status = storage.CrawlJobStatusPartialFailed
	case job.Failed > 0:
		job.Status = storage.CrawlJobStatusFailed
	default:
		job.Status = storage.CrawlJobStatusSucceeded
	}
	now := time.Now()
	job.FinishedAt = &now
	s.jobs[jobID] = job
	return job, nil
}

func (s *fakeCrawlJobStore) FailCrawlJob(_ context.Context, jobID int64, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[jobID]
	job.Status = storage.CrawlJobStatusFailed
	job.Error = message
	now := time.Now()
	job.FinishedAt = &now
	s.jobs[jobID] = job
	return nil
}

func (s *fakeCrawlJobStore) GetCrawlJobDetail(_ context.Context, jobID int64) (*storage.CrawlJobDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return nil, nil
	}
	failed := []storage.CrawlJobItem{}
	for _, item := range s.items[jobID] {
		if item.Status == storage.CrawlJobItemStatusFailed {
			failed = append(failed, item)
		}
	}
	return &storage.CrawlJobDetail{Job: job, FailedItems: failed}, nil
}

func (s *fakeCrawlJobStore) updateItem(itemID int64, mutate func(*storage.CrawlJobItem)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for jobID, items := range s.items {
		for i := range items {
			if items[i].ID == itemID {
				mutate(&items[i])
				s.items[jobID] = items
				return nil
			}
		}
	}
	return errors.New("item not found")
}

func waitForCrawlJobStatus(t *testing.T, store *fakeCrawlJobStore, jobID int64, status string) storage.CrawlJobDetail {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := store.GetCrawlJobDetail(context.Background(), jobID)
		if err != nil {
			t.Fatalf("GetCrawlJobDetail() error = %v", err)
		}
		if detail != nil && detail.Job.Status == status {
			return *detail
		}
		time.Sleep(10 * time.Millisecond)
	}
	detail, _ := store.GetCrawlJobDetail(context.Background(), jobID)
	t.Fatalf("job did not reach %s: %+v", status, detail)
	return storage.CrawlJobDetail{}
}
