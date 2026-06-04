package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"leetcodeclaw/internal/leetcode"
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

type fakeProblemService struct {
	candidates    []leetcode.SearchCandidate
	problems      map[string]leetcode.Problem
	searchOptions leetcode.SearchOptions
}

func (s *fakeProblemService) SearchProblems(_ context.Context, options leetcode.SearchOptions) ([]leetcode.SearchCandidate, error) {
	s.searchOptions = options
	return s.candidates, nil
}

func (s *fakeProblemService) CrawlProblem(_ context.Context, slug string) (leetcode.Problem, error) {
	return s.problems[slug], nil
}
