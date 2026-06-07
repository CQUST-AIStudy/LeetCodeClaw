package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"leetcodeclaw/internal/leetcode"
	"leetcodeclaw/internal/storage"
)

type Store interface {
	Ping(context.Context) error
	CheckSchema(context.Context) error
	UpsertProblem(context.Context, leetcode.Problem) (storage.PersistResult, error)
	FindProblemBySlug(context.Context, string) (*leetcode.Problem, error)
	FindActiveCrawlJob(context.Context) (*storage.CrawlJob, error)
	CreateCrawlJob(context.Context, storage.CrawlJobConfig) (storage.CrawlJob, error)
	AddCrawlJobItems(context.Context, int64, []string) error
	MarkCrawlJobRunning(context.Context, int64, int) error
	ListCrawlJobItemsByStatus(context.Context, int64, string) ([]storage.CrawlJobItem, error)
	MarkCrawlJobItemRunning(context.Context, int64) error
	MarkCrawlJobItemSucceeded(context.Context, int64, int64) error
	MarkCrawlJobItemFailed(context.Context, int64, string) error
	FinishCrawlJob(context.Context, int64) (storage.CrawlJob, error)
	FailCrawlJob(context.Context, int64, string) error
	GetCrawlJobDetail(context.Context, int64) (*storage.CrawlJobDetail, error)
}

type ProblemService interface {
	SearchProblems(context.Context, leetcode.SearchOptions) ([]leetcode.SearchCandidate, error)
	ListPublicProblems(context.Context, leetcode.PublicProblemListOptions) ([]leetcode.SearchCandidate, error)
	CrawlProblem(context.Context, string) (leetcode.Problem, error)
}

type CrawlAllConfig struct {
	Workers  int
	PageSize int
	Delay    time.Duration
}

type ServerConfig struct {
	CrawlAll    CrawlAllConfig
	APIKey      string
	CORSOrigins []string
}

type Server struct {
	service     ProblemService
	store       Store
	crawlAll    CrawlAllConfig
	apiKey      string
	corsOrigins []string
	jobMu       sync.Mutex
}

func NewServer(service ProblemService, store Store) *Server {
	return NewServerWithCrawlAllConfig(service, store, CrawlAllConfig{
		Workers:  1,
		PageSize: 100,
		Delay:    2 * time.Second,
	})
}

func NewServerWithCrawlAllConfig(service ProblemService, store Store, crawlAll CrawlAllConfig) *Server {
	return NewServerWithConfig(service, store, ServerConfig{CrawlAll: crawlAll})
}

func NewServerWithConfig(service ProblemService, store Store, config ServerConfig) *Server {
	return &Server{
		service:     service,
		store:       normalizeStore(store),
		crawlAll:    normalizeCrawlAllConfig(config.CrawlAll),
		apiKey:      strings.TrimSpace(config.APIKey),
		corsOrigins: normalizeCORSOrigins(config.CORSOrigins),
	}
}

func normalizeStore(store Store) Store {
	if store == nil {
		return nil
	}
	if mysqlStore, ok := store.(*storage.Store); ok && mysqlStore == nil {
		return nil
	}
	return store
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/api/leetcode/crawl/all", s.handleCrawlAll)
	mux.HandleFunc("/api/leetcode/crawl/jobs/", s.handleCrawlJobByID)
	mux.HandleFunc("/api/leetcode/crawl", s.handleCrawl)
	mux.HandleFunc("/api/leetcode/recommend/keyword", s.handleKeywordRecommend)
	mux.HandleFunc("/api/leetcode/problem/", s.handleProblemBySlug)
	return withCORS(withAPIKey(mux, s.apiKey), s.corsOrigins)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbStatus, schemaStatus, _ := s.databaseStatus(ctx)

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"service":  "leetcode-claw-api",
		"database": dbStatus,
		"schema":   schemaStatus,
		"leetcode": "configured",
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbStatus, schemaStatus, ready := s.databaseStatus(ctx)
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"success":  ready,
		"service":  "leetcode-claw-api",
		"database": dbStatus,
		"schema":   schemaStatus,
	})
}

func (s *Server) databaseStatus(ctx context.Context) (string, string, bool) {
	if s.store == nil {
		return "disabled", "disabled", false
	}
	if err := s.store.Ping(ctx); err != nil {
		return err.Error(), "skipped", false
	}
	if err := s.store.CheckSchema(ctx); err != nil {
		return "ok", err.Error(), false
	}
	return "ok", "ok", true
}

type crawlRequest struct {
	Slugs   []string `json:"slugs"`
	Persist *bool    `json:"persist,omitempty"`
}

type crawlResponse struct {
	Success bool             `json:"success"`
	Items   []crawlItem      `json:"items"`
	Failed  []failedResponse `json:"failed"`
}

type crawlItem struct {
	Slug      string                 `json:"slug"`
	Problem   leetcode.Problem       `json:"problem"`
	Solution  leetcode.Solution      `json:"solution"`
	Persisted bool                   `json:"persisted"`
	Persist   *storage.PersistResult `json:"persist,omitempty"`
	Warnings  []string               `json:"warnings,omitempty"`
	Errors    []string               `json:"errors,omitempty"`
}

type failedResponse struct {
	Slug  string `json:"slug,omitempty"`
	Error string `json:"error"`
}

type crawlAllRequest struct {
	Persist      *bool `json:"persist,omitempty"`
	ForceRefresh *bool `json:"forceRefresh,omitempty"`
}

type crawlAllResponse struct {
	Success bool   `json:"success"`
	JobID   int64  `json:"jobId"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type crawlJobResponse struct {
	Success bool                  `json:"success"`
	Job     crawlJobView          `json:"job"`
	Failed  []crawlJobItemFailure `json:"failed,omitempty"`
}

type crawlJobView struct {
	ID           int64      `json:"id"`
	Status       string     `json:"status"`
	Persist      bool       `json:"persist"`
	ForceRefresh bool       `json:"forceRefresh"`
	Workers      int        `json:"workers"`
	DelayMillis  int64      `json:"delayMillis"`
	PageSize     int        `json:"pageSize"`
	Total        int        `json:"total"`
	Succeeded    int        `json:"succeeded"`
	Failed       int        `json:"failed"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}

type crawlJobItemFailure struct {
	Slug     string `json:"slug"`
	Error    string `json:"error"`
	Attempts int    `json:"attempts"`
}

func (s *Server) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req crawlRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	slugs := normalizeSlugs(req.Slugs)
	if len(slugs) == 0 {
		writeError(w, http.StatusBadRequest, "slugs is required")
		return
	}
	persist := boolDefault(req.Persist, true)

	resp := crawlResponse{Success: true}
	for _, slug := range slugs {
		item, err := s.crawlOne(r.Context(), slug, persist)
		if err != nil {
			resp.Failed = append(resp.Failed, failedResponse{Slug: slug, Error: err.Error()})
			continue
		}
		resp.Items = append(resp.Items, item)
	}
	if len(resp.Failed) > 0 {
		resp.Success = len(resp.Items) > 0
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCrawlAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database store is required for crawl-all jobs")
		return
	}
	if s.service == nil {
		writeError(w, http.StatusServiceUnavailable, "leetcode service is not configured")
		return
	}

	var req crawlAllRequest
	if err := decodeOptionalJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	persist := boolDefault(req.Persist, true)
	forceRefresh := boolDefault(req.ForceRefresh, true)
	cfg := normalizeCrawlAllConfig(s.crawlAll)

	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	active, err := s.store.FindActiveCrawlJob(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if active != nil {
		writeJSON(w, http.StatusConflict, crawlAllResponse{
			Success: false,
			JobID:   active.ID,
			Status:  active.Status,
			Message: "crawl-all job is already running",
		})
		return
	}

	job, err := s.store.CreateCrawlJob(r.Context(), storage.CrawlJobConfig{
		Persist:      persist,
		ForceRefresh: forceRefresh,
		Workers:      cfg.Workers,
		Delay:        cfg.Delay,
		PageSize:     cfg.PageSize,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.runCrawlAllJob(job)

	writeJSON(w, http.StatusAccepted, crawlAllResponse{
		Success: true,
		JobID:   job.ID,
		Status:  job.Status,
	})
}

func (s *Server) handleCrawlJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database store is required for crawl jobs")
		return
	}

	rawID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/leetcode/crawl/jobs/"), "/")
	if rawID == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}
	jobID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || jobID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	detail, err := s.store.GetCrawlJobDetail(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "crawl job not found")
		return
	}

	writeJSON(w, http.StatusOK, crawlJobResponse{
		Success: true,
		Job:     crawlJobViewFromStorage(detail.Job),
		Failed:  crawlJobFailuresFromStorage(detail.FailedItems),
	})
}

type keywordRecommendRequest struct {
	Keyword    string `json:"keyword"`
	Limit      int    `json:"limit"`
	Difficulty string `json:"difficulty,omitempty"`
	Persist    *bool  `json:"persist,omitempty"`
}

type keywordRecommendResponse struct {
	Success bool                   `json:"success"`
	Keyword string                 `json:"keyword"`
	Items   []keywordRecommendItem `json:"items"`
	Failed  []failedResponse       `json:"failed"`
	Omitted []omittedRecommendItem `json:"omitted,omitempty"`
}

type keywordRecommendItem struct {
	Rank      int                    `json:"rank"`
	Score     float64                `json:"score"`
	Reason    string                 `json:"reason"`
	Problem   leetcode.Problem       `json:"problem"`
	Solution  leetcode.Solution      `json:"solution"`
	Persisted bool                   `json:"persisted"`
	Persist   *storage.PersistResult `json:"persist,omitempty"`
	Warnings  []string               `json:"warnings,omitempty"`
	Errors    []string               `json:"errors,omitempty"`
}

type omittedRecommendItem struct {
	Slug   string   `json:"slug"`
	Title  string   `json:"title"`
	Reason string   `json:"reason"`
	Errors []string `json:"errors,omitempty"`
}

func (s *Server) handleKeywordRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req keywordRecommendRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		writeError(w, http.StatusBadRequest, "keyword is required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 50 {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 50")
		return
	}
	persist := boolDefault(req.Persist, true)

	candidates, err := s.service.SearchProblems(r.Context(), leetcode.SearchOptions{
		Keyword:    req.Keyword,
		Limit:      expandedRecommendLimit(req.Limit),
		Difficulty: req.Difficulty,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	resp := keywordRecommendResponse{
		Success: true,
		Keyword: req.Keyword,
	}
	for _, candidate := range candidates {
		if len(resp.Items) >= req.Limit {
			break
		}
		item, err := s.crawlOne(r.Context(), candidate.TitleSlug, persist)
		if err != nil {
			resp.Failed = append(resp.Failed, failedResponse{Slug: candidate.TitleSlug, Error: err.Error()})
			continue
		}
		if isMissingSolutionContent(item) {
			resp.Omitted = append(resp.Omitted, omittedRecommendItem{
				Slug:   firstNonEmptyText(item.Slug, candidate.TitleSlug),
				Title:  firstNonEmptyText(item.Problem.TranslatedTitle, item.Problem.Title, candidate.TranslatedTitle, candidate.Title),
				Reason: "题解正文缺失",
				Errors: item.Errors,
			})
			continue
		}
		resp.Items = append(resp.Items, keywordRecommendItem{
			Rank:      len(resp.Items) + 1,
			Score:     candidate.Score,
			Reason:    enrichReason(candidate.Reason, item),
			Problem:   item.Problem,
			Solution:  item.Solution,
			Persisted: item.Persisted,
			Persist:   item.Persist,
			Warnings:  item.Warnings,
			Errors:    item.Errors,
		})
	}
	if len(resp.Failed) > 0 {
		resp.Success = len(resp.Items) > 0
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleProblemBySlug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/leetcode/problem/"), "/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	if s.store != nil {
		problem, err := s.store.FindProblemBySlug(r.Context(), slug)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if problem != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true,
				"source":  "database",
				"problem": problem,
			})
			return
		}
	}

	shouldCrawl, _ := strconv.ParseBool(r.URL.Query().Get("crawl"))
	if !shouldCrawl {
		writeError(w, http.StatusNotFound, "problem not found")
		return
	}
	item, err := s.crawlOne(r.Context(), slug, true)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"source":  "leetcode.cn",
		"item":    item,
	})
}

func (s *Server) runCrawlAllJob(job storage.CrawlJob) {
	ctx := context.Background()
	candidates, err := s.service.ListPublicProblems(ctx, leetcode.PublicProblemListOptions{PageSize: job.PageSize})
	if err != nil {
		_ = s.store.FailCrawlJob(ctx, job.ID, err.Error())
		return
	}

	slugs := slugsFromCandidates(candidates)
	if err := s.store.AddCrawlJobItems(ctx, job.ID, slugs); err != nil {
		_ = s.store.FailCrawlJob(ctx, job.ID, err.Error())
		return
	}
	if err := s.store.MarkCrawlJobRunning(ctx, job.ID, len(slugs)); err != nil {
		_ = s.store.FailCrawlJob(ctx, job.ID, err.Error())
		return
	}

	items, err := s.store.ListCrawlJobItemsByStatus(ctx, job.ID, storage.CrawlJobItemStatusPending)
	if err != nil {
		_ = s.store.FailCrawlJob(ctx, job.ID, err.Error())
		return
	}

	workerCount := job.Workers
	if workerCount <= 0 {
		workerCount = 1
	}
	itemCh := make(chan storage.CrawlJobItem)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemCh {
				s.processCrawlJobItem(ctx, job, item)
				if job.Delay > 0 {
					time.Sleep(job.Delay)
				}
			}
		}()
	}
	for _, item := range items {
		itemCh <- item
	}
	close(itemCh)
	wg.Wait()

	if _, err := s.store.FinishCrawlJob(ctx, job.ID); err != nil {
		_ = s.store.FailCrawlJob(ctx, job.ID, err.Error())
	}
}

func (s *Server) processCrawlJobItem(ctx context.Context, job storage.CrawlJob, item storage.CrawlJobItem) {
	if err := s.store.MarkCrawlJobItemRunning(ctx, item.ID); err != nil {
		return
	}
	if !job.ForceRefresh && job.Persist {
		existing, err := s.store.FindProblemBySlug(ctx, item.Slug)
		if err != nil {
			_ = s.store.MarkCrawlJobItemFailed(ctx, item.ID, err.Error())
			return
		}
		if existing != nil {
			_ = s.store.MarkCrawlJobItemSucceeded(ctx, item.ID, 0)
			return
		}
	}

	problem, err := s.service.CrawlProblem(ctx, item.Slug)
	if err != nil {
		_ = s.store.MarkCrawlJobItemFailed(ctx, item.ID, err.Error())
		return
	}
	var problemID int64
	if job.Persist {
		result, err := s.persistProblem(ctx, problem)
		if err != nil {
			_ = s.store.MarkCrawlJobItemFailed(ctx, item.ID, err.Error())
			return
		}
		problemID = result.ProblemID
	}
	_ = s.store.MarkCrawlJobItemSucceeded(ctx, item.ID, problemID)
}

func (s *Server) crawlOne(ctx context.Context, slug string, persist bool) (crawlItem, error) {
	if !validSlug(slug) {
		return crawlItem{}, fmt.Errorf("invalid slug %q", slug)
	}
	problem, err := s.service.CrawlProblem(ctx, slug)
	if err != nil {
		return crawlItem{}, err
	}
	item := crawlItem{
		Slug:     problem.TitleSlug,
		Problem:  problem,
		Solution: problem.Solution,
		Warnings: problem.Errors,
	}
	if err := leetcode.ValidateSolutionComplete(problem); err != nil {
		item.Errors = append(item.Errors, err.Error())
	}
	if persist {
		result, err := s.persistProblem(ctx, problem)
		if err != nil {
			item.Errors = append(item.Errors, "persist failed: "+err.Error())
			return item, nil
		}
		item.Persisted = true
		item.Persist = &result
	}
	return item, nil
}

func (s *Server) persistProblem(ctx context.Context, problem leetcode.Problem) (storage.PersistResult, error) {
	if s.store == nil {
		return storage.PersistResult{}, errors.New("database store is not configured")
	}
	if strings.TrimSpace(problem.Solution.ContentMarkdown) == "" {
		return storage.PersistResult{}, errors.New("solution content is empty")
	}
	return s.store.UpsertProblem(ctx, problem)
}

func enrichReason(reason string, item crawlItem) string {
	reasons := []string{}
	if strings.TrimSpace(reason) != "" {
		reasons = append(reasons, strings.TrimSpace(reason))
	}
	if item.Persisted {
		reasons = append(reasons, "已写入题库")
	}
	if len(item.Errors) == 0 {
		reasons = append(reasons, "题目与题解完整")
	}
	if len(reasons) == 0 {
		return "关键词候选推荐"
	}
	return strings.Join(reasons, "，")
}

func expandedRecommendLimit(limit int) int {
	expanded := limit * 3
	if expanded < limit {
		return limit
	}
	if expanded > 50 {
		return 50
	}
	return expanded
}

func isMissingSolutionContent(item crawlItem) bool {
	return strings.TrimSpace(item.Problem.Solution.ContentMarkdown) == ""
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func slugsFromCandidates(candidates []leetcode.SearchCandidate) []string {
	slugs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.TitleSlug) != "" {
			slugs = append(slugs, candidate.TitleSlug)
		}
	}
	return normalizeSlugs(slugs)
}

func crawlJobViewFromStorage(job storage.CrawlJob) crawlJobView {
	return crawlJobView{
		ID:           job.ID,
		Status:       job.Status,
		Persist:      job.Persist,
		ForceRefresh: job.ForceRefresh,
		Workers:      job.Workers,
		DelayMillis:  int64(job.Delay / time.Millisecond),
		PageSize:     job.PageSize,
		Total:        job.Total,
		Succeeded:    job.Succeeded,
		Failed:       job.Failed,
		Error:        job.Error,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		FinishedAt:   job.FinishedAt,
	}
}

func crawlJobFailuresFromStorage(items []storage.CrawlJobItem) []crawlJobItemFailure {
	if len(items) == 0 {
		return nil
	}
	failures := make([]crawlJobItemFailure, 0, len(items))
	for _, item := range items {
		failures = append(failures, crawlJobItemFailure{
			Slug:     item.Slug,
			Error:    item.Error,
			Attempts: item.Attempts,
		})
	}
	return failures
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"success": false,
		"message": message,
	})
}

func withCORS(next http.Handler, origins []string) http.Handler {
	origins = normalizeCORSOrigins(origins)
	allowAll := len(origins) == 1 && origins[0] == "*"
	allowed := map[string]bool{}
	for _, origin := range origins {
		if origin != "*" {
			allowed[origin] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withAPIKey(next http.Handler, apiKey string) http.Handler {
	apiKey = strings.TrimSpace(apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" || r.Method == http.MethodOptions || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if requestHasAPIKey(r, apiKey) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="leetcode-claw-api"`)
		writeError(w, http.StatusUnauthorized, "missing or invalid API key")
	})
}

func requestHasAPIKey(r *http.Request, expected string) bool {
	for _, candidate := range []string{bearerToken(r.Header.Get("Authorization")), r.Header.Get("X-API-Key")} {
		if constantTimeEqual(strings.TrimSpace(candidate), expected) {
			return true
		}
	}
	return false
}

func bearerToken(header string) string {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}

func constantTimeEqual(candidate, expected string) bool {
	if candidate == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1
}

func normalizeCORSOrigins(origins []string) []string {
	seen := map[string]bool{}
	normalized := []string{}
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		normalized = append(normalized, origin)
	}
	if len(normalized) == 0 {
		return []string{"*"}
	}
	return normalized
}

func normalizeSlugs(values []string) []string {
	seen := map[string]bool{}
	slugs := []string{}
	for _, value := range values {
		slug := strings.TrimSpace(value)
		if slug == "" {
			continue
		}
		key := strings.ToLower(slug)
		if seen[key] {
			continue
		}
		seen[key] = true
		slugs = append(slugs, slug)
	}
	return slugs
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizeCrawlAllConfig(config CrawlAllConfig) CrawlAllConfig {
	if config.Workers <= 0 {
		config.Workers = 1
	}
	if config.PageSize <= 0 {
		config.PageSize = 100
	}
	if config.PageSize > 200 {
		config.PageSize = 200
	}
	if config.Delay < 0 {
		config.Delay = 0
	}
	return config
}

func validSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, r := range slug {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			continue
		}
		return false
	}
	return true
}
