package storage

import (
	"strings"
	"testing"

	"leetcodeclaw/internal/leetcode"
)

func TestProblemRecordFromLeetCode(t *testing.T) {
	problem := leetcode.Problem{
		QuestionFrontendID: "102",
		Title:              "Binary Tree Level Order Traversal",
		TitleSlug:          "binary-tree-level-order-traversal",
		TranslatedTitle:    "二叉树的层序遍历",
		Difficulty:         "Medium",
		ContentMarkdown:    "题面",
		Solution: leetcode.Solution{
			ContentMarkdown: "题解",
			CodeByLanguage: map[string]string{
				"cpp": "class Solution {};",
			},
		},
	}

	record := ProblemRecordFromLeetCode(problem)
	if record.SourceKey != "slug:binary-tree-level-order-traversal" {
		t.Fatalf("SourceKey = %q", record.SourceKey)
	}
	if record.NumericID == nil || *record.NumericID != 102 {
		t.Fatalf("NumericID = %v", record.NumericID)
	}
	if record.TitleMain != "二叉树的层序遍历" {
		t.Fatalf("TitleMain = %q", record.TitleMain)
	}
	if record.Difficulty != "Medium" {
		t.Fatalf("Difficulty = %q", record.Difficulty)
	}
	if record.EstimatedMinutes != 35 {
		t.Fatalf("EstimatedMinutes = %d", record.EstimatedMinutes)
	}
	if !strings.Contains(record.SolutionText, "class Solution") {
		t.Fatalf("SolutionText missing code: %q", record.SolutionText)
	}
}

func TestTagsFromLeetCodeDeduplicatesAndCategorizes(t *testing.T) {
	tags := TagsFromLeetCode([]leetcode.TopicTag{
		{Name: "Tree", Slug: "tree"},
		{Name: "Tree", Slug: "tree"},
		{Name: "Dynamic Programming", Slug: "dynamic-programming"},
	})
	if len(tags) != 2 {
		t.Fatalf("len(tags) = %d, want 2", len(tags))
	}
	if tags[0].Type != "data_structure" || tags[0].Confidence != 0.95 {
		t.Fatalf("first tag = %+v", tags[0])
	}
	if tags[1].Type != "technique" {
		t.Fatalf("second tag = %+v", tags[1])
	}
}
