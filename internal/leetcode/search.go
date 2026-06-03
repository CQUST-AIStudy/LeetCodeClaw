package leetcode

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const problemsetQuestionListQuery = `
query problemsetQuestionList($categorySlug: String, $limit: Int, $skip: Int, $filters: QuestionListFilterInput) {
  problemsetQuestionList(
    categorySlug: $categorySlug
    limit: $limit
    skip: $skip
    filters: $filters
  ) {
    total
    questions {
      frontendQuestionId
      title
      titleCn
      titleSlug
      difficulty
      topicTags {
        name
        nameTranslated
        slug
      }
    }
  }
}`

type problemsetQuestionListData struct {
	ProblemsetQuestionList *problemsetQuestionList `json:"problemsetQuestionList"`
}

type problemsetQuestionList struct {
	Total     int                  `json:"total"`
	Questions []problemsetQuestion `json:"questions"`
}

type problemsetQuestion struct {
	QuestionID         string         `json:"questionId"`
	QuestionFrontendID string         `json:"questionFrontendId"`
	FrontendQuestionID string         `json:"frontendQuestionId"`
	Title              string         `json:"title"`
	TitleCn            string         `json:"titleCn"`
	TitleSlug          string         `json:"titleSlug"`
	TranslatedTitle    string         `json:"translatedTitle"`
	Difficulty         string         `json:"difficulty"`
	TopicTags          []lightTagNode `json:"topicTags"`
}

type lightTagNode struct {
	Name           string `json:"name"`
	NameTranslated string `json:"nameTranslated"`
	Slug           string `json:"slug"`
}

func (s *ProblemService) SearchProblems(ctx context.Context, options SearchOptions) ([]SearchCandidate, error) {
	keyword := strings.TrimSpace(options.Keyword)
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	fetchLimit := limit * 3
	if fetchLimit < 20 {
		fetchLimit = 20
	}
	if fetchLimit > 100 {
		fetchLimit = 100
	}

	filters := map[string]any{
		"searchKeywords": keyword,
	}
	if difficulty := NormalizeDifficulty(options.Difficulty); difficulty != "Unknown" {
		filters["difficulty"] = strings.ToUpper(difficulty)
	}

	var data problemsetQuestionListData
	err := s.client.doGraphQL(ctx, "https://leetcode.cn/problemset/", graphQLRequest{
		OperationName: "problemsetQuestionList",
		Query:         problemsetQuestionListQuery,
		Variables: map[string]any{
			"categorySlug": "",
			"skip":         0,
			"limit":        fetchLimit,
			"filters":      filters,
		},
	}, &data)
	if err != nil {
		return nil, fmt.Errorf("search problems: %w", err)
	}
	if data.ProblemsetQuestionList == nil {
		return nil, nil
	}

	candidates := make([]SearchCandidate, 0, len(data.ProblemsetQuestionList.Questions))
	for _, question := range data.ProblemsetQuestionList.Questions {
		if strings.TrimSpace(question.TitleSlug) == "" {
			continue
		}
		candidate := SearchCandidate{
			QuestionID:         question.QuestionID,
			QuestionFrontendID: firstNonEmpty(question.QuestionFrontendID, question.FrontendQuestionID),
			Title:              question.Title,
			TitleSlug:          question.TitleSlug,
			TranslatedTitle:    firstNonEmpty(question.TranslatedTitle, question.TitleCn),
			Difficulty:         NormalizeDifficulty(question.Difficulty),
			Tags:               convertLightTags(question.TopicTags),
		}
		candidate.Score, candidate.Reason = ScoreCandidate(keyword, NormalizeDifficulty(options.Difficulty), candidate)
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].TitleSlug < candidates[j].TitleSlug
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func convertLightTags(tags []lightTagNode) []TopicTag {
	result := make([]TopicTag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, TopicTag{
			Name:           tag.Name,
			TranslatedName: tag.NameTranslated,
			Slug:           tag.Slug,
		})
	}
	return result
}

func ScoreCandidate(keyword, difficulty string, candidate SearchCandidate) (float64, string) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	difficulty = NormalizeDifficulty(difficulty)

	score := 0.30
	reasons := []string{}
	titleText := strings.ToLower(candidate.Title + " " + candidate.TranslatedTitle + " " + candidate.TitleSlug)
	if keyword != "" && strings.Contains(titleText, keyword) {
		score += 0.35
		reasons = append(reasons, "标题与关键词匹配")
	}
	if tagMatchesKeyword(keyword, candidate.Tags) {
		score += 0.25
		reasons = append(reasons, "标签与关键词匹配")
	}
	if difficulty != "Unknown" && NormalizeDifficulty(candidate.Difficulty) == difficulty {
		score += 0.15
		reasons = append(reasons, "难度符合筛选条件")
	}
	if len(candidate.Tags) > 0 {
		score += 0.05
		reasons = append(reasons, "题目标签完整")
	}
	if score > 1 {
		score = 1
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "关键词搜索候选题")
	}
	return score, strings.Join(reasons, "，")
}

func tagMatchesKeyword(keyword string, tags []TopicTag) bool {
	if keyword == "" {
		return false
	}
	for _, tag := range tags {
		text := strings.ToLower(tag.Name + " " + tag.TranslatedName + " " + tag.Slug)
		if strings.Contains(text, keyword) || strings.Contains(keyword, strings.ToLower(tag.Slug)) {
			return true
		}
	}
	return false
}

func NormalizeDifficulty(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "easy", "简单":
		return "Easy"
	case "medium", "中等":
		return "Medium"
	case "hard", "困难":
		return "Hard"
	default:
		return "Unknown"
	}
}
