package leetcode

import (
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
