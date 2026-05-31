package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"leetcodeclaw/internal/leetcode"
)

type Format string

const (
	FormatMarkdown Format = "md"
	FormatJSON     Format = "json"
)

type Renderer struct {
	outDir  string
	formats map[Format]bool
}

func NewRenderer(outDir string, formats map[Format]bool) *Renderer {
	return &Renderer{outDir: outDir, formats: formats}
}

func ParseFormats(raw string) (map[Format]bool, error) {
	result := map[Format]bool{}
	for _, item := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		switch Format(value) {
		case FormatMarkdown, FormatJSON:
			result[Format(value)] = true
		default:
			return nil, fmt.Errorf("unsupported format %q", value)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("at least one format is required")
	}
	return result, nil
}

func (r *Renderer) WriteProblem(problem leetcode.Problem) error {
	if strings.TrimSpace(problem.TitleSlug) == "" {
		return errors.New("problem titleSlug is empty")
	}
	if !validTitleSlug(problem.TitleSlug) {
		return fmt.Errorf("invalid problem titleSlug %q", problem.TitleSlug)
	}

	dir := filepath.Join(r.outDir, problem.TitleSlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if r.formats[FormatMarkdown] {
		if err := os.WriteFile(filepath.Join(dir, "problem.md"), []byte(RenderProblemMarkdown(problem)), 0o644); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
	}

	if r.formats[FormatJSON] {
		data, err := json.MarshalIndent(problem, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join(dir, "problem.json"), data, 0o644); err != nil {
			return fmt.Errorf("write json: %w", err)
		}
	}

	return nil
}

func (r *Renderer) DeleteProblem(titleSlug string) error {
	titleSlug = strings.TrimSpace(titleSlug)
	if titleSlug == "" {
		return nil
	}
	if !validTitleSlug(titleSlug) {
		return fmt.Errorf("invalid problem titleSlug %q", titleSlug)
	}

	baseAbs, err := filepath.Abs(r.outDir)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(r.outDir, titleSlug))
	if err != nil {
		return fmt.Errorf("resolve problem directory: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return fmt.Errorf("check problem directory: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("refuse to delete path outside output directory: %s", targetAbs)
	}
	if err := os.RemoveAll(targetAbs); err != nil {
		return fmt.Errorf("delete problem output: %w", err)
	}
	return nil
}

func RenderProblemMarkdown(problem leetcode.Problem) string {
	var b strings.Builder

	title := problem.TranslatedTitle
	if title == "" {
		title = problem.Title
	}
	fmt.Fprintf(&b, "# %s. %s\n\n", problem.QuestionFrontendID, title)
	fmt.Fprintf(&b, "- Slug: `%s`\n", problem.TitleSlug)
	fmt.Fprintf(&b, "- Difficulty: `%s`\n", problem.Difficulty)
	if len(problem.Tags) > 0 {
		fmt.Fprintf(&b, "- Tags: %s\n", renderTags(problem.Tags))
	}
	b.WriteString("\n")

	b.WriteString("## 题目\n\n")
	b.WriteString(strings.TrimSpace(problem.ContentMarkdown))
	b.WriteString("\n\n")

	b.WriteString("## 初始化代码\n\n")
	for _, snippet := range problem.CodeSnippets {
		lang := markdownFenceLanguage(snippet.LangSlug)
		fmt.Fprintf(&b, "### %s\n\n", snippet.Lang)
		fmt.Fprintf(&b, "```%s\n%s\n```\n\n", lang, strings.TrimSpace(snippet.Code))
	}

	b.WriteString("## 题解\n\n")
	if problem.Solution.Title != "" {
		fmt.Fprintf(&b, "- Title: %s\n", problem.Solution.Title)
	}
	fmt.Fprintf(&b, "- Source: %s\n", problem.Solution.Source)
	if problem.Solution.SourceSlug != "" {
		fmt.Fprintf(&b, "- Source Slug: `%s`\n", problem.Solution.SourceSlug)
	}
	if problem.Solution.FallbackReason != "" {
		fmt.Fprintf(&b, "- Fallback: %s\n", problem.Solution.FallbackReason)
	}
	b.WriteString("\n")
	if strings.TrimSpace(problem.Solution.ContentMarkdown) != "" {
		b.WriteString(strings.TrimSpace(problem.Solution.ContentMarkdown))
		b.WriteString("\n\n")
	}

	for _, lang := range []string{"c", "cpp"} {
		code := strings.TrimSpace(problem.Solution.CodeByLanguage[lang])
		if code == "" {
			continue
		}
		fmt.Fprintf(&b, "### %s 题解代码\n\n", displayLanguage(lang))
		fmt.Fprintf(&b, "```%s\n%s\n```\n\n", lang, code)
	}

	if len(problem.Solution.MissingLanguages) > 0 {
		b.WriteString("### 题解缺失语言\n\n")
		keys := make([]string, 0, len(problem.Solution.MissingLanguages))
		for key := range problem.Solution.MissingLanguages {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "- `%s`: %s\n", key, problem.Solution.MissingLanguages[key])
		}
		b.WriteString("\n")
	}

	if len(problem.Errors) > 0 {
		b.WriteString("## 抓取警告\n\n")
		for _, item := range problem.Errors {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func validTitleSlug(slug string) bool {
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

func renderTags(tags []leetcode.TopicTag) string {
	values := make([]string, 0, len(tags))
	for _, tag := range tags {
		name := tag.TranslatedName
		if name == "" {
			name = tag.Name
		}
		if tag.Slug != "" {
			values = append(values, fmt.Sprintf("%s (`%s`)", name, tag.Slug))
		} else {
			values = append(values, name)
		}
	}
	return strings.Join(values, ", ")
}

func markdownFenceLanguage(langSlug string) string {
	switch strings.ToLower(langSlug) {
	case "c":
		return "c"
	case "cpp":
		return "cpp"
	default:
		return strings.ToLower(langSlug)
	}
}

func displayLanguage(lang string) string {
	switch lang {
	case "c":
		return "C"
	case "cpp":
		return "C++"
	default:
		return lang
	}
}
