package leetcode

import "testing"

func TestFilterCodeSnippetsMapsWantedLanguages(t *testing.T) {
	input := []codeSnippetNode{
		{Lang: "C", LangSlug: "c", Code: "int* twoSum(int* nums, int numsSize, int target, int* returnSize) {}"},
		{Lang: "Java", LangSlug: "java", Code: "class Solution {}"},
		{Lang: "Python3", LangSlug: "python3", Code: "class Solution:"},
		{Lang: "C++", LangSlug: "cpp", Code: "class Solution {};"},
		{Lang: "Go", LangSlug: "golang", Code: "func twoSum() {}"},
		{Lang: "JavaScript", LangSlug: "javascript", Code: "var twoSum = function() {};"},
		{Lang: "Python", LangSlug: "python", Code: ""},
	}

	got := filterCodeSnippets(input)
	if len(got) != 6 {
		t.Fatalf("expected 6 snippets, got %d: %+v", len(got), got)
	}

	seen := map[string]bool{}
	for _, snippet := range got {
		seen[snippet.LangSlug] = true
	}
	for _, langSlug := range []string{"java", "python3", "c", "cpp", "javascript", "golang"} {
		if !seen[langSlug] {
			t.Fatalf("missing snippet langSlug %q in %+v", langSlug, got)
		}
	}
	if got[0].LangSlug != "java" || got[1].LangSlug != "python3" {
		t.Fatalf("supported snippets should be ordered first, got %+v", got)
	}
}

func TestBuildSolutionRecordsMissingLanguages(t *testing.T) {
	solution, warnings := buildSolution("题解", "solution-slug", "two-sum", `<pre><code class="language-c">int main() { return 0; }</code></pre>`, "test")
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if solution.CodeByLanguage["c"] == "" {
		t.Fatalf("expected c code")
	}
	if solution.MissingLanguages["cpp"] == "" {
		t.Fatalf("expected missing cpp language reason")
	}
}

func TestEmptySolutionMarksAllLanguagesMissing(t *testing.T) {
	solution := emptySolution("two-sum", "不可见")
	for _, lang := range []string{"c", "cpp"} {
		if solution.MissingLanguages[lang] != "不可见" {
			t.Fatalf("missing reason for %s = %q", lang, solution.MissingLanguages[lang])
		}
	}
}

func TestExtractEquivalentProblemSlugs(t *testing.T) {
	raw := `<p>同主站 <a href="https://leetcode.cn/problems/linked-list-cycle-ii/">142</a></p>` +
		`<a href="/problems/two-sum/description/">two sum</a>` +
		`<a href="https://leetcode.cn/problems/c32eOV/">self</a>` +
		`<a href="https://leetcode.cn/problems/linked-list-cycle-ii/">dup</a>`

	got := extractEquivalentProblemSlugs(raw, "c32eov")
	want := []string{"linked-list-cycle-ii", "two-sum"}
	if len(got) != len(want) {
		t.Fatalf("expected %d slugs, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slug %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateSolutionCompleteFailsWhenLanguageMissing(t *testing.T) {
	problem := Problem{
		TitleSlug: "two-sum",
		Solution: Solution{
			SourceSlug:      "two-sum",
			ContentMarkdown: "content",
			CodeByLanguage:  map[string]string{},
		},
	}
	if err := ValidateSolutionComplete(problem); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateSolutionCompletePassesWhenCIsPresent(t *testing.T) {
	problem := Problem{
		TitleSlug: "two-sum",
		Solution: Solution{
			SourceSlug:      "two-sum",
			ContentMarkdown: "content",
			CodeByLanguage: map[string]string{
				"c": "int main() { return 0; }",
			},
		},
	}
	if err := ValidateSolutionComplete(problem); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateSolutionCompletePassesWhenCPPIsPresent(t *testing.T) {
	problem := Problem{
		TitleSlug: "two-sum",
		Solution: Solution{
			SourceSlug:      "two-sum",
			ContentMarkdown: "content",
			CodeByLanguage: map[string]string{
				"cpp": "class Solution {};",
			},
		},
	}
	if err := ValidateSolutionComplete(problem); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestExtractLanguageCodeNormalizesCPPFence(t *testing.T) {
	got := extractLanguageCode("```C++ []\nclass Solution {};\n```")
	if got["cpp"] == "" {
		t.Fatalf("expected cpp code, got %v", got)
	}
}
