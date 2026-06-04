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
		CodeSnippets: []leetcode.CodeSnippet{
			{Lang: "Java", LangSlug: "java", Code: "class Solution { public int[] levelOrder(TreeNode root) { return new int[0]; } }"},
			{Lang: "C++", LangSlug: "cpp", Code: "class Solution { public: int getNumber(TreeNode* root) { return 0; } };"},
		},
		ContentMarkdown: "题面",
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
	if !strings.Contains(record.CodeSnippetsJSON, "getNumber") {
		t.Fatalf("CodeSnippetsJSON missing starter code: %q", record.CodeSnippetsJSON)
	}
	if !strings.Contains(record.CodeSnippetsJSON, "levelOrder") {
		t.Fatalf("CodeSnippetsJSON missing java starter code: %q", record.CodeSnippetsJSON)
	}
	decoded := decodeCodeSnippets(record.CodeSnippetsJSON)
	if len(decoded) != 2 || decoded[0].LangSlug != "java" || decoded[1].LangSlug != "cpp" {
		t.Fatalf("decoded snippets = %+v", decoded)
	}
	if !strings.Contains(decoded[0].Code, "levelOrder") || !strings.Contains(decoded[1].Code, "getNumber") {
		t.Fatalf("decoded snippets = %+v", decoded)
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
	if tags[0].Type != "data_structure" || tags[0].Confidence != 0.95 || !tags[0].IsPrimary {
		t.Fatalf("first tag = %+v", tags[0])
	}
	if tags[1].Type != "technique" || tags[1].IsPrimary {
		t.Fatalf("second tag = %+v", tags[1])
	}
}
