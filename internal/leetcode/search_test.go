package leetcode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeDifficulty(t *testing.T) {
	cases := map[string]string{
		"easy":    "Easy",
		"简单":      "Easy",
		"MEDIUM":  "Medium",
		"中等":      "Medium",
		"Hard":    "Hard",
		"困难":      "Hard",
		"unknown": "Unknown",
	}
	for input, want := range cases {
		if got := NormalizeDifficulty(input); got != want {
			t.Fatalf("NormalizeDifficulty(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestScoreCandidateUsesTitleTagAndDifficulty(t *testing.T) {
	candidate := SearchCandidate{
		Title:           "Binary Tree Level Order Traversal",
		TranslatedTitle: "二叉树的层序遍历",
		Difficulty:      "Medium",
		Tags: []TopicTag{
			{Name: "Tree", TranslatedName: "树", Slug: "tree"},
			{Name: "Breadth-First Search", TranslatedName: "广度优先搜索", Slug: "breadth-first-search"},
		},
	}

	score, reason := ScoreCandidate("二叉树", "Medium", candidate)
	if score < 0.75 {
		t.Fatalf("score = %v, want >= 0.75", score)
	}
	for _, part := range []string{"标题与关键词匹配", "难度符合筛选条件"} {
		if !strings.Contains(reason, part) {
			t.Fatalf("reason %q missing %q", reason, part)
		}
	}
}

func TestListPublicProblemsPaginatesAndFilters(t *testing.T) {
	requests := []int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		skip, _ := payload.Variables["skip"].(float64)
		requests = append(requests, int(skip))

		var questions []map[string]any
		switch int(skip) {
		case 0:
			questions = []map[string]any{
				{
					"frontendQuestionId": "1",
					"title":              "Two Sum",
					"titleCn":            "两数之和",
					"titleSlug":          "two-sum",
					"difficulty":         "Easy",
					"paidOnly":           false,
				},
				{
					"frontendQuestionId": "2",
					"title":              "Paid Problem",
					"titleSlug":          "paid-problem",
					"difficulty":         "Medium",
					"paidOnly":           true,
				},
			}
		case 2:
			questions = []map[string]any{
				{
					"frontendQuestionId": "1",
					"title":              "Two Sum Duplicate",
					"titleSlug":          "two-sum",
					"difficulty":         "Easy",
					"paidOnly":           false,
				},
				{
					"frontendQuestionId": "3",
					"title":              "Add Two Numbers",
					"titleCn":            "两数相加",
					"titleSlug":          "add-two-numbers",
					"difficulty":         "Medium",
					"paidOnly":           false,
				},
			}
		case 4:
			questions = []map[string]any{
				{
					"frontendQuestionId": "4",
					"title":              "Empty Slug",
					"titleSlug":          "",
					"difficulty":         "Hard",
					"paidOnly":           false,
				},
			}
		default:
			questions = nil
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"problemsetQuestionList": map[string]any{
					"total":     5,
					"questions": questions,
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client(), 0)
	client.endpoint = server.URL
	service := NewProblemService(client)

	got, err := service.ListPublicProblems(context.Background(), PublicProblemListOptions{PageSize: 2})
	if err != nil {
		t.Fatalf("ListPublicProblems() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].TitleSlug != "two-sum" || got[1].TitleSlug != "add-two-numbers" {
		t.Fatalf("slugs = %+v", got)
	}
	if len(requests) != 3 || requests[0] != 0 || requests[1] != 2 || requests[2] != 4 {
		t.Fatalf("requests = %v", requests)
	}
}
