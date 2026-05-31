package leetcode

type Problem struct {
	QuestionID         string        `json:"questionId"`
	QuestionFrontendID string        `json:"questionFrontendId"`
	Title              string        `json:"title"`
	TitleSlug          string        `json:"titleSlug"`
	TranslatedTitle    string        `json:"translatedTitle,omitempty"`
	Difficulty         string        `json:"difficulty"`
	Tags               []TopicTag    `json:"tags"`
	ContentMarkdown    string        `json:"contentMarkdown"`
	CodeSnippets       []CodeSnippet `json:"codeSnippets"`
	Solution           Solution      `json:"solution"`
	Errors             []string      `json:"errors,omitempty"`
}

type TopicTag struct {
	Name           string `json:"name"`
	TranslatedName string `json:"translatedName,omitempty"`
	Slug           string `json:"slug"`
}

type CodeSnippet struct {
	Lang     string `json:"lang"`
	LangSlug string `json:"langSlug"`
	Code     string `json:"code"`
}

type Solution struct {
	Source           string            `json:"source"`
	SourceSlug       string            `json:"sourceSlug,omitempty"`
	FallbackReason   string            `json:"fallbackReason,omitempty"`
	Title            string            `json:"title,omitempty"`
	ArticleSlug      string            `json:"articleSlug,omitempty"`
	ContentMarkdown  string            `json:"contentMarkdown,omitempty"`
	CodeByLanguage   map[string]string `json:"codeByLanguage"`
	MissingLanguages map[string]string `json:"missingLanguages,omitempty"`
}

type CrawlResult struct {
	OutputDir string          `json:"outputDir"`
	Succeeded []string        `json:"succeeded"`
	Failed    []FailedProblem `json:"failed"`
}

type FailedProblem struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

func wantedLanguages() map[string]string {
	return map[string]string{
		"c":   "c",
		"cpp": "cpp",
		"c++": "cpp",
		"cc":  "cpp",
		"cxx": "cpp",
	}
}

func canonicalLanguage(langSlug string) (string, bool) {
	value, ok := wantedLanguages()[langSlug]
	return value, ok
}

func targetLanguages() []string {
	return []string{"c", "cpp"}
}
