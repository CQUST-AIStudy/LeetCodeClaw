package leetcode

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"leetcodeclaw/internal/htmlmd"
)

const questionDataQuery = `
query questionData($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    questionId
    questionFrontendId
    title
    titleSlug
    translatedTitle
    translatedContent
    content
    difficulty
    topicTags {
      name
      translatedName
      slug
    }
    codeSnippets {
      lang
      langSlug
      code
    }
  }
}`

const questionSolutionInfoQuery = `
query questionSolutionInfo($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    solution {
      id
      title
      content
    }
  }
}`

const communitySolutionListQuery = `
query questionTopicsList($questionSlug: String!, $skip: Int, $first: Int, $orderBy: SolutionArticleOrderBy, $tagSlugs: [String!]) {
  questionSolutionArticles(
    questionSlug: $questionSlug
    skip: $skip
    first: $first
    orderBy: $orderBy
    tagSlugs: $tagSlugs
  ) {
    totalNum
    edges {
      node {
        uuid
        title
        slug
        canSee
        hasVideo
        upvoteCount
        hitCount
        byLeetcode
        isEditorsPick
        isMostPopular
        tags {
          name
          nameTranslated
          slug
          tagType
        }
      }
    }
  }
}`

const communitySolutionDetailQuery = `
query discussTopic($slug: String) {
  solutionArticle(slug: $slug, orderBy: DEFAULT) {
    title
    slug
    content
    canSee
    hasVideo
    byLeetcode
    tags {
      name
      nameTranslated
      slug
      tagType
    }
  }
}`

type ProblemService struct {
	client *Client
}

func NewProblemService(client *Client) *ProblemService {
	return &ProblemService{client: client}
}

type questionData struct {
	Question *questionNode `json:"question"`
}

type questionNode struct {
	QuestionID         string            `json:"questionId"`
	QuestionFrontendID string            `json:"questionFrontendId"`
	Title              string            `json:"title"`
	TitleSlug          string            `json:"titleSlug"`
	TranslatedTitle    string            `json:"translatedTitle"`
	TranslatedContent  string            `json:"translatedContent"`
	Content            string            `json:"content"`
	Difficulty         string            `json:"difficulty"`
	TopicTags          []topicTagNode    `json:"topicTags"`
	CodeSnippets       []codeSnippetNode `json:"codeSnippets"`
	Solution           *solutionInfoNode `json:"solution,omitempty"`
}

type topicTagNode struct {
	Name           string `json:"name"`
	TranslatedName string `json:"translatedName"`
	Slug           string `json:"slug"`
}

type codeSnippetNode struct {
	Lang     string `json:"lang"`
	LangSlug string `json:"langSlug"`
	Code     string `json:"code"`
}

type solutionInfoData struct {
	Question *struct {
		Solution *solutionInfoNode `json:"solution"`
	} `json:"question"`
}

type solutionInfoNode struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type communitySolutionListData struct {
	QuestionSolutionArticles *communitySolutionConnection `json:"questionSolutionArticles"`
}

type communitySolutionConnection struct {
	TotalNum int                     `json:"totalNum"`
	Edges    []communitySolutionEdge `json:"edges"`
}

type communitySolutionEdge struct {
	Node communitySolutionNode `json:"node"`
}

type communitySolutionNode struct {
	UUID          string            `json:"uuid"`
	Title         string            `json:"title"`
	Slug          string            `json:"slug"`
	CanSee        bool              `json:"canSee"`
	HasVideo      bool              `json:"hasVideo"`
	UpvoteCount   int               `json:"upvoteCount"`
	HitCount      int               `json:"hitCount"`
	ByLeetcode    bool              `json:"byLeetcode"`
	IsEditorsPick bool              `json:"isEditorsPick"`
	IsMostPopular bool              `json:"isMostPopular"`
	Tags          []solutionTagNode `json:"tags"`
}

type communitySolutionDetailData struct {
	SolutionArticle *communitySolutionDetailNode `json:"solutionArticle"`
}

type communitySolutionDetailNode struct {
	Title      string            `json:"title"`
	Slug       string            `json:"slug"`
	Content    string            `json:"content"`
	CanSee     bool              `json:"canSee"`
	HasVideo   bool              `json:"hasVideo"`
	ByLeetcode bool              `json:"byLeetcode"`
	Tags       []solutionTagNode `json:"tags"`
}

type solutionTagNode struct {
	Name           string `json:"name"`
	NameTranslated string `json:"nameTranslated"`
	Slug           string `json:"slug"`
	TagType        string `json:"tagType"`
}

func (s *ProblemService) CrawlProblem(ctx context.Context, titleSlug string) (Problem, error) {
	titleSlug = strings.TrimSpace(titleSlug)
	if titleSlug == "" {
		return Problem{}, errors.New("empty title slug")
	}

	question, err := s.fetchQuestion(ctx, titleSlug)
	if err != nil {
		return Problem{}, err
	}
	if question == nil {
		return Problem{}, fmt.Errorf("question not found: %s", titleSlug)
	}

	contentHTML := firstNonEmpty(question.TranslatedContent, question.Content)
	equivalentSlugs := extractEquivalentProblemSlugs(contentHTML, question.TitleSlug)
	contentMarkdown, err := htmlmd.Convert(contentHTML)
	if err != nil {
		contentMarkdown = strings.TrimSpace(contentHTML)
	}

	problem := Problem{
		QuestionID:         question.QuestionID,
		QuestionFrontendID: question.QuestionFrontendID,
		Title:              question.Title,
		TitleSlug:          question.TitleSlug,
		TranslatedTitle:    question.TranslatedTitle,
		Difficulty:         question.Difficulty,
		Tags:               convertTags(question.TopicTags),
		ContentMarkdown:    contentMarkdown,
		CodeSnippets:       filterCodeSnippets(question.CodeSnippets),
		Solution: Solution{
			Source:           "leetcode.cn official",
			CodeByLanguage:   map[string]string{},
			MissingLanguages: map[string]string{},
		},
	}

	if err != nil {
		problem.Errors = append(problem.Errors, fmt.Sprintf("题面 HTML 转换失败，已回退为原始内容: %v", err))
	}
	appendMissingSnippets(&problem)

	solution, warnings := s.fetchSolutionWithFallback(ctx, question.TitleSlug, equivalentSlugs)
	problem.Errors = append(problem.Errors, warnings...)
	if solution != nil {
		problem.Solution = *solution
	}
	appendMissingSolutionLanguages(&problem)

	return problem, nil
}

func (s *ProblemService) fetchQuestion(ctx context.Context, titleSlug string) (*questionNode, error) {
	var data questionData
	err := s.client.doGraphQL(ctx, problemReferer(titleSlug), graphQLRequest{
		OperationName: "questionData",
		Query:         questionDataQuery,
		Variables: map[string]any{
			"titleSlug": titleSlug,
		},
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("fetch question %s: %w", titleSlug, err)
	}
	return data.Question, nil
}

func (s *ProblemService) fetchSolutionWithFallback(ctx context.Context, titleSlug string, fallbackSlugs []string) (*Solution, []string) {
	solution, warnings := s.fetchOfficialSolutionWithFallback(ctx, titleSlug, fallbackSlugs)
	if isCompleteSolution(solution) {
		return solution, warnings
	}

	best := solution
	slugs := append([]string{titleSlug}, fallbackSlugs...)
	for _, slug := range slugs {
		community, communityWarnings := s.fetchCommunitySolution(ctx, slug)
		warnings = append(warnings, communityWarnings...)
		if slug != titleSlug && community != nil {
			community.FallbackReason = fmt.Sprintf("当前题 %s 官方题解不完整，使用题面等价题 %s 的社区题解", titleSlug, slug)
		}
		if isCompleteSolution(community) {
			return community, warnings
		}
		if solutionScore(community) > solutionScore(best) {
			best = community
		}
	}

	if !isCompleteSolution(best) {
		warnings = append(warnings, "官方题解和公开社区题解均未提供 C/C++ 可用代码块")
	}
	return best, warnings
}

func (s *ProblemService) fetchOfficialSolutionWithFallback(ctx context.Context, titleSlug string, fallbackSlugs []string) (*Solution, []string) {
	primary, primaryWarnings := s.fetchOfficialSolution(ctx, titleSlug)
	if isCompleteSolution(primary) {
		return primary, primaryWarnings
	}

	best := primary
	for _, fallbackSlug := range fallbackSlugs {
		fallback, fallbackWarnings := s.fetchOfficialSolution(ctx, fallbackSlug)
		if len(fallbackWarnings) > 0 {
			primaryWarnings = append(primaryWarnings, prefixWarnings(fmt.Sprintf("等价题 %s", fallbackSlug), fallbackWarnings)...)
		}
		if fallback != nil {
			fallback.FallbackReason = fmt.Sprintf("当前题 %s 官方题解不完整，使用题面等价题 %s", titleSlug, fallbackSlug)
		}
		if isCompleteSolution(fallback) {
			return fallback, []string{fmt.Sprintf("当前题官方题解不完整，已回退到等价题 %s", fallbackSlug)}
		}
		if solutionScore(fallback) > solutionScore(best) {
			best = fallback
			primaryWarnings = append(primaryWarnings, fmt.Sprintf("等价题 %s 官方题解仍不完整", fallbackSlug))
		}
	}

	if len(fallbackSlugs) > 0 && !isCompleteSolution(best) {
		primaryWarnings = append(primaryWarnings, fmt.Sprintf("已尝试题面等价题 %s，未获得 C/C++ 官方题解", strings.Join(fallbackSlugs, ", ")))
	}
	return best, primaryWarnings
}

func (s *ProblemService) fetchOfficialSolution(ctx context.Context, titleSlug string) (*Solution, []string) {
	var warnings []string
	info, err := s.fetchQuestionSolutionInfo(ctx, titleSlug)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("官方题解信息抓取失败: %v", err))
		return emptySolution(titleSlug, "官方题解信息抓取失败"), warnings
	}
	if info == nil {
		warnings = append(warnings, "官方题解不存在或公开接口未返回题解信息")
		return emptySolution(titleSlug, "官方题解不存在或公开接口未返回题解信息"), warnings
	}
	if strings.TrimSpace(info.Content) != "" {
		solution, moreWarnings := buildSolution(info.Title, "", titleSlug, info.Content, "leetcode.cn official question solution")
		warnings = append(warnings, moreWarnings...)
		return solution, warnings
	}

	warnings = append(warnings, "官方题解未返回公开正文")
	return emptySolution(titleSlug, "官方题解未返回公开正文"), warnings
}

func (s *ProblemService) fetchQuestionSolutionInfo(ctx context.Context, titleSlug string) (*solutionInfoNode, error) {
	var data solutionInfoData
	err := s.client.doGraphQL(ctx, problemReferer(titleSlug), graphQLRequest{
		OperationName: "questionSolutionInfo",
		Query:         questionSolutionInfoQuery,
		Variables: map[string]any{
			"titleSlug": titleSlug,
		},
	}, &data)
	if err != nil {
		return nil, err
	}
	if data.Question == nil {
		return nil, nil
	}
	return data.Question.Solution, nil
}

func (s *ProblemService) fetchCommunitySolution(ctx context.Context, titleSlug string) (*Solution, []string) {
	candidates, err := s.fetchCommunitySolutionCandidates(ctx, titleSlug)
	if err != nil {
		return nil, []string{fmt.Sprintf("社区题解列表抓取失败: %v", err)}
	}
	if len(candidates) == 0 {
		return nil, []string{fmt.Sprintf("社区题解列表为空或无 C/C++ 候选: %s", titleSlug)}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return communityCandidateScore(candidates[i]) > communityCandidateScore(candidates[j])
	})

	var warnings []string
	var best *Solution
	for _, candidate := range candidates {
		if !candidate.CanSee {
			warnings = append(warnings, fmt.Sprintf("社区题解不可见，已跳过: %s", candidate.Slug))
			continue
		}
		detail, err := s.fetchCommunitySolutionDetail(ctx, titleSlug, candidate.Slug)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("社区题解正文抓取失败 %s: %v", candidate.Slug, err))
			continue
		}
		if detail == nil {
			warnings = append(warnings, fmt.Sprintf("社区题解正文为空: %s", candidate.Slug))
			continue
		}
		if detail.HasVideo && strings.TrimSpace(detail.Content) == "" {
			warnings = append(warnings, fmt.Sprintf("社区题解为纯视频内容，已跳过: %s", candidate.Slug))
			continue
		}

		solution, moreWarnings := buildSolution(detail.Title, detail.Slug, titleSlug, detail.Content, "leetcode.cn community solution")
		warnings = append(warnings, moreWarnings...)
		if solution != nil {
			solution.FallbackReason = "官方题解不完整，使用公开社区题解"
		}
		if isCompleteSolution(solution) {
			return solution, warnings
		}
		if solutionScore(solution) > solutionScore(best) {
			best = solution
		}
	}

	if best == nil {
		warnings = append(warnings, fmt.Sprintf("未找到可用社区题解正文: %s", titleSlug))
	}
	return best, warnings
}

func (s *ProblemService) fetchCommunitySolutionCandidates(ctx context.Context, titleSlug string) ([]communitySolutionNode, error) {
	candidateMap := map[string]communitySolutionNode{}
	for _, tagSlugs := range [][]string{{"c", "cpp"}, {"cpp"}, {"c"}, nil} {
		var data communitySolutionListData
		variables := map[string]any{
			"questionSlug": titleSlug,
			"skip":         0,
			"first":        12,
			"orderBy":      "DEFAULT",
		}
		if tagSlugs != nil {
			variables["tagSlugs"] = tagSlugs
		}
		err := s.client.doGraphQL(ctx, problemSolutionsReferer(titleSlug), graphQLRequest{
			OperationName: "questionTopicsList",
			Query:         communitySolutionListQuery,
			Variables:     variables,
		}, &data)
		if err != nil {
			return nil, err
		}
		if data.QuestionSolutionArticles == nil {
			continue
		}
		for _, edge := range data.QuestionSolutionArticles.Edges {
			node := edge.Node
			if strings.TrimSpace(node.Slug) == "" {
				continue
			}
			if !hasTargetLanguageTag(node.Tags) && len(tagSlugs) == 0 {
				continue
			}
			if existing, ok := candidateMap[node.Slug]; !ok || communityCandidateScore(node) > communityCandidateScore(existing) {
				candidateMap[node.Slug] = node
			}
		}
	}

	candidates := make([]communitySolutionNode, 0, len(candidateMap))
	for _, candidate := range candidateMap {
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (s *ProblemService) fetchCommunitySolutionDetail(ctx context.Context, titleSlug, articleSlug string) (*communitySolutionDetailNode, error) {
	var data communitySolutionDetailData
	err := s.client.doGraphQL(ctx, problemSolutionsReferer(titleSlug), graphQLRequest{
		OperationName: "discussTopic",
		Query:         communitySolutionDetailQuery,
		Variables: map[string]any{
			"slug": articleSlug,
		},
	}, &data)
	if err != nil {
		return nil, err
	}
	return data.SolutionArticle, nil
}

func buildSolution(title, articleSlug, sourceSlug, rawHTML, source string) (*Solution, []string) {
	markdown, err := htmlmd.Convert(rawHTML)
	warnings := []string{}
	if err != nil {
		markdown = strings.TrimSpace(rawHTML)
		warnings = append(warnings, fmt.Sprintf("官方题解 HTML 转换失败，已回退为原始内容: %v", err))
	}

	codeByLanguage := extractLanguageCode(markdown)
	solution := &Solution{
		Source:           source,
		SourceSlug:       sourceSlug,
		Title:            title,
		ArticleSlug:      articleSlug,
		ContentMarkdown:  markdown,
		CodeByLanguage:   codeByLanguage,
		MissingLanguages: map[string]string{},
	}
	for _, lang := range targetLanguages() {
		if strings.TrimSpace(solution.CodeByLanguage[lang]) == "" {
			solution.MissingLanguages[lang] = "官方题解正文中未找到对应语言代码块"
		}
	}
	return solution, warnings
}

func emptySolution(sourceSlug, reason string) *Solution {
	missing := map[string]string{}
	for _, lang := range targetLanguages() {
		missing[lang] = reason
	}
	return &Solution{
		Source:           "leetcode.cn official",
		SourceSlug:       sourceSlug,
		CodeByLanguage:   map[string]string{},
		MissingLanguages: missing,
	}
}

func convertTags(tags []topicTagNode) []TopicTag {
	result := make([]TopicTag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, TopicTag{
			Name:           tag.Name,
			TranslatedName: tag.TranslatedName,
			Slug:           tag.Slug,
		})
	}
	return result
}

func filterCodeSnippets(snippets []codeSnippetNode) []CodeSnippet {
	result := make([]CodeSnippet, 0, 4)
	seen := map[string]bool{}
	for _, snippet := range snippets {
		canonical, ok := canonicalLanguage(strings.ToLower(snippet.LangSlug))
		if !ok {
			continue
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		result = append(result, CodeSnippet{
			Lang:     snippet.Lang,
			LangSlug: snippet.LangSlug,
			Code:     snippet.Code,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return languageOrder(result[i].LangSlug) < languageOrder(result[j].LangSlug)
	})
	return result
}

func appendMissingSnippets(problem *Problem) {
	present := map[string]bool{}
	for _, snippet := range problem.CodeSnippets {
		if canonical, ok := canonicalLanguage(strings.ToLower(snippet.LangSlug)); ok {
			present[canonical] = true
		}
	}
	for _, lang := range targetLanguages() {
		if !present[lang] {
			problem.Errors = append(problem.Errors, fmt.Sprintf("初始化代码缺失: %s", lang))
		}
	}
}

func appendMissingSolutionLanguages(problem *Problem) {
	if problem.Solution.MissingLanguages == nil {
		problem.Solution.MissingLanguages = map[string]string{}
	}
	for _, lang := range targetLanguages() {
		if strings.TrimSpace(problem.Solution.CodeByLanguage[lang]) == "" {
			if _, ok := problem.Solution.MissingLanguages[lang]; !ok {
				problem.Solution.MissingLanguages[lang] = "官方题解正文中未找到对应语言代码块"
			}
		}
	}
}

func ValidateSolutionComplete(problem Problem) error {
	if strings.TrimSpace(problem.Solution.ContentMarkdown) == "" {
		return fmt.Errorf("题解正文缺失，来源题目: %s", fallbackString(problem.Solution.SourceSlug, problem.TitleSlug))
	}

	missing := missingSolutionLanguages(problem.Solution)
	if len(missing) == len(targetLanguages()) {
		return fmt.Errorf("题解不完整，缺失 C/C++ 题解代码，来源题目: %s", fallbackString(problem.Solution.SourceSlug, problem.TitleSlug))
	}
	return nil
}

func FormatProblemWarnings(warnings []string) string {
	cleaned := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning != "" {
			cleaned = append(cleaned, warning)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return strings.Join(cleaned, "; ")
}

func missingSolutionLanguages(solution Solution) []string {
	missing := []string{}
	for _, lang := range targetLanguages() {
		if strings.TrimSpace(solution.CodeByLanguage[lang]) == "" {
			missing = append(missing, lang)
		}
	}
	return missing
}

func isCompleteSolution(solution *Solution) bool {
	if solution == nil {
		return false
	}
	if strings.TrimSpace(solution.ContentMarkdown) == "" {
		return false
	}
	return len(missingSolutionLanguages(*solution)) < len(targetLanguages())
}

func solutionScore(solution *Solution) int {
	if solution == nil {
		return 0
	}
	score := 0
	if strings.TrimSpace(solution.ContentMarkdown) != "" {
		score += 10
	}
	for _, code := range solution.CodeByLanguage {
		if strings.TrimSpace(code) != "" {
			score++
		}
	}
	return score
}

func prefixWarnings(prefix string, warnings []string) []string {
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning != "" {
			result = append(result, fmt.Sprintf("%s: %s", prefix, warning))
		}
	}
	return result
}

func languageOrder(langSlug string) int {
	canonical, ok := canonicalLanguage(strings.ToLower(langSlug))
	if !ok {
		return 99
	}
	switch canonical {
	case "c":
		return 0
	case "cpp":
		return 1
	default:
		return 99
	}
}

func extractLanguageCode(markdown string) map[string]string {
	result := map[string]string{}
	re := regexp.MustCompile("(?s)```([^\\n`]*)\\n(.*?)```")
	matches := re.FindAllStringSubmatch(markdown, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		langInfo := strings.ToLower(strings.TrimSpace(match[1]))
		code := strings.TrimSpace(match[2])
		if code == "" {
			continue
		}
		for _, token := range strings.Fields(langInfo) {
			if canonical, ok := canonicalLanguage(normalizeFenceLanguage(token)); ok {
				if result[canonical] == "" {
					result[canonical] = code
				}
				break
			}
		}
	}
	return result
}

func extractEquivalentProblemSlugs(rawContent, currentSlug string) []string {
	re := regexp.MustCompile(`(?i)(?:https?://leetcode\.cn)?/problems/([a-z0-9-]+)/?`)
	matches := re.FindAllStringSubmatch(rawContent, -1)
	result := make([]string, 0, len(matches))
	seen := map[string]bool{}
	currentSlug = strings.ToLower(strings.TrimSpace(currentSlug))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		slug := strings.TrimSpace(match[1])
		key := strings.ToLower(slug)
		if key == "" || key == currentSlug || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, slug)
	}
	return result
}

func normalizeFenceLanguage(language string) string {
	language = strings.Trim(strings.ToLower(language), " \t\r\n{}[]()")
	switch language {
	case "c":
		return "c"
	case "c++", "cpp", "cc", "cxx":
		return "cpp"
	default:
		return language
	}
}

func hasTargetLanguageTag(tags []solutionTagNode) bool {
	for _, tag := range tags {
		if _, ok := canonicalLanguage(strings.ToLower(tag.Slug)); ok {
			return true
		}
	}
	return false
}

func communityCandidateScore(candidate communitySolutionNode) int {
	score := candidate.UpvoteCount*10 + candidate.HitCount/1000
	if hasTargetLanguageTag(candidate.Tags) {
		score += 1_000_000
	}
	if candidate.IsEditorsPick {
		score += 100_000
	}
	if candidate.IsMostPopular {
		score += 50_000
	}
	if candidate.HasVideo {
		score -= 1_000
	}
	return score
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func problemReferer(titleSlug string) string {
	return fmt.Sprintf("https://leetcode.cn/problems/%s/", titleSlug)
}

func problemSolutionsReferer(titleSlug string) string {
	return fmt.Sprintf("https://leetcode.cn/problems/%s/solutions/", titleSlug)
}
