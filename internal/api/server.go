package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"leetcodeclaw/internal/leetcode"
	"leetcodeclaw/internal/storage"
)

type Store interface {
	Ping(context.Context) error
	CheckSchema(context.Context) error
	UpsertProblem(context.Context, leetcode.Problem) (storage.PersistResult, error)
	FindProblemBySlug(context.Context, string) (*leetcode.Problem, error)
}

type Server struct {
	service *leetcode.ProblemService
	store   Store
}

func NewServer(service *leetcode.ProblemService, store Store) *Server {
	return &Server{service: service, store: store}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/leetcode/crawl", s.handleCrawl)
	mux.HandleFunc("/api/leetcode/recommend/keyword", s.handleKeywordRecommend)
	mux.HandleFunc("/api/leetcode/problem/", s.handleProblemBySlug)
	return withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbStatus := "ok"
	schemaStatus := "ok"
	if s.store == nil {
		dbStatus = "disabled"
		schemaStatus = "disabled"
	} else if err := s.store.Ping(ctx); err != nil {
		dbStatus = err.Error()
		schemaStatus = "skipped"
	} else if err := s.store.CheckSchema(ctx); err != nil {
		schemaStatus = err.Error()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"service":  "leetcode-claw-api",
		"database": dbStatus,
		"schema":   schemaStatus,
		"leetcode": "configured",
	})
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
		Limit:      req.Limit,
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
		item, err := s.crawlOne(r.Context(), candidate.TitleSlug, persist)
		if err != nil {
			resp.Failed = append(resp.Failed, failedResponse{Slug: candidate.TitleSlug, Error: err.Error()})
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

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
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

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
