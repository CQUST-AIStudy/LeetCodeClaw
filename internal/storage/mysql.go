package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"leetcodeclaw/internal/leetcode"
)

type Store struct {
	db *sql.DB
}

type PersistResult struct {
	ProblemID int64 `json:"problemId"`
	Inserted  bool  `json:"inserted"`
}

func NewMySQLStore(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("mysql dsn is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) CheckSchema(ctx context.Context) error {
	if err := s.requireColumns(ctx, "leetcode_problem_bank",
		"source_key", "problem_code", "numeric_id", "title_main", "title_alt",
		"problem_text", "solution_text", "source_url", "difficulty", "estimated_minutes", "quality_score",
	); err != nil {
		return err
	}
	if err := s.requireColumns(ctx, "leetcode_problem_tag", "problem_id", "tag_type", "tag_value", "confidence"); err != nil {
		return fmt.Errorf("%w; please align leetcode_problem_tag with Java Mapper fields tag_type/tag_value/confidence", err)
	}
	return nil
}

func (s *Store) UpsertProblem(ctx context.Context, problem leetcode.Problem) (PersistResult, error) {
	if s == nil || s.db == nil {
		return PersistResult{}, errors.New("store is not initialized")
	}
	if strings.TrimSpace(problem.TitleSlug) == "" {
		return PersistResult{}, errors.New("problem titleSlug is empty")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PersistResult{}, err
	}
	defer tx.Rollback()

	record := ProblemRecordFromLeetCode(problem)
	var existingID sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id FROM leetcode_problem_bank WHERE source_key = ? LIMIT 1`, record.SourceKey).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PersistResult{}, err
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO leetcode_problem_bank (
  source_key, problem_code, numeric_id, title_main, title_alt,
  problem_text, solution_text, source_url, difficulty, estimated_minutes, quality_score
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  id = LAST_INSERT_ID(id),
  problem_code = VALUES(problem_code),
  numeric_id = VALUES(numeric_id),
  title_main = VALUES(title_main),
  title_alt = VALUES(title_alt),
  problem_text = VALUES(problem_text),
  solution_text = VALUES(solution_text),
  source_url = VALUES(source_url),
  difficulty = VALUES(difficulty),
  estimated_minutes = VALUES(estimated_minutes),
  quality_score = VALUES(quality_score),
  updated_at = CURRENT_TIMESTAMP`,
		record.SourceKey,
		nullString(record.ProblemCode),
		nullInt(record.NumericID),
		record.TitleMain,
		nullString(record.TitleAlt),
		record.ProblemText,
		record.SolutionText,
		nullString(record.SourceURL),
		record.Difficulty,
		record.EstimatedMinutes,
		record.QualityScore,
	)
	if err != nil {
		return PersistResult{}, err
	}
	problemID, err := result.LastInsertId()
	if err != nil {
		return PersistResult{}, err
	}
	if problemID == 0 && existingID.Valid {
		problemID = existingID.Int64
	}
	if problemID == 0 {
		return PersistResult{}, errors.New("failed to resolve persisted problem id")
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM leetcode_problem_tag WHERE problem_id = ?`, problemID); err != nil {
		return PersistResult{}, err
	}
	for _, tag := range TagsFromLeetCode(problem.Tags) {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO leetcode_problem_tag (problem_id, tag_type, tag_value, confidence) VALUES (?, ?, ?, ?)`,
			problemID, tag.Type, tag.Value, tag.Confidence,
		); err != nil {
			return PersistResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return PersistResult{}, err
	}
	return PersistResult{
		ProblemID: problemID,
		Inserted:  !existingID.Valid,
	}, nil
}

func (s *Store) FindProblemBySlug(ctx context.Context, slug string) (*leetcode.Problem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, errors.New("slug is required")
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id, problem_code, numeric_id, title_main, title_alt, problem_text, solution_text, source_url, difficulty
FROM leetcode_problem_bank
WHERE source_key = ? OR source_url = ?
LIMIT 1`, "slug:"+slug, problemURL(slug))

	var id int64
	var problemCode, titleAlt, sourceURL sql.NullString
	var numericID sql.NullInt64
	var titleMain, problemText, solutionText, difficulty string
	if err := row.Scan(&id, &problemCode, &numericID, &titleMain, &titleAlt, &problemText, &solutionText, &sourceURL, &difficulty); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	tags, err := s.findTags(ctx, id)
	if err != nil {
		return nil, err
	}
	problem := &leetcode.Problem{
		QuestionFrontendID: problemCode.String,
		Title:              titleAlt.String,
		TitleSlug:          slug,
		TranslatedTitle:    titleMain,
		Difficulty:         leetcode.NormalizeDifficulty(difficulty),
		Tags:               tags,
		ContentMarkdown:    problemText,
		Solution: leetcode.Solution{
			Source:          "local database",
			SourceSlug:      slug,
			ContentMarkdown: solutionText,
			CodeByLanguage:  map[string]string{},
		},
	}
	return problem, nil
}

func (s *Store) findTags(ctx context.Context, problemID int64) ([]leetcode.TopicTag, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT tag_type, tag_value FROM leetcode_problem_tag WHERE problem_id = ? ORDER BY confidence DESC, id ASC`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []leetcode.TopicTag{}
	for rows.Next() {
		var tagType, tagValue string
		if err := rows.Scan(&tagType, &tagValue); err != nil {
			return nil, err
		}
		tags = append(tags, leetcode.TopicTag{
			Name: tagValue,
			Slug: tagValue,
		})
	}
	return tags, rows.Err()
}

func (s *Store) requireColumns(ctx context.Context, tableName string, columns ...string) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		found[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(found) == 0 {
		return fmt.Errorf("table %s is missing", tableName)
	}
	missing := []string{}
	for _, column := range columns {
		if !found[strings.ToLower(column)] {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("table %s missing columns: %s", tableName, strings.Join(missing, ", "))
	}
	return nil
}

type ProblemRecord struct {
	SourceKey        string
	ProblemCode      string
	NumericID        *int
	TitleMain        string
	TitleAlt         string
	ProblemText      string
	SolutionText     string
	SourceURL        string
	Difficulty       string
	EstimatedMinutes int
	QualityScore     float64
}

type ProblemTagRecord struct {
	Type       string
	Value      string
	Confidence float64
}

func ProblemRecordFromLeetCode(problem leetcode.Problem) ProblemRecord {
	titleMain := strings.TrimSpace(problem.TranslatedTitle)
	if titleMain == "" {
		titleMain = strings.TrimSpace(problem.Title)
	}
	titleAlt := strings.TrimSpace(problem.Title)
	numericID := numericQuestionID(problem.QuestionFrontendID)
	return ProblemRecord{
		SourceKey:        "slug:" + problem.TitleSlug,
		ProblemCode:      strings.TrimSpace(problem.QuestionFrontendID),
		NumericID:        numericID,
		TitleMain:        titleMain,
		TitleAlt:         titleAlt,
		ProblemText:      strings.TrimSpace(problem.ContentMarkdown),
		SolutionText:     buildSolutionText(problem.Solution),
		SourceURL:        problemURL(problem.TitleSlug),
		Difficulty:       leetcode.NormalizeDifficulty(problem.Difficulty),
		EstimatedMinutes: estimatedMinutes(problem.Difficulty),
		QualityScore:     qualityScore(problem),
	}
}

func TagsFromLeetCode(tags []leetcode.TopicTag) []ProblemTagRecord {
	seen := map[string]bool{}
	result := make([]ProblemTagRecord, 0, len(tags))
	for i, tag := range tags {
		value := strings.TrimSpace(tag.Slug)
		if value == "" {
			value = strings.TrimSpace(tag.Name)
		}
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		confidence := 0.80
		if i == 0 {
			confidence = 0.95
		}
		result = append(result, ProblemTagRecord{
			Type:       tagCategory(value),
			Value:      value,
			Confidence: confidence,
		})
	}
	if len(result) == 0 {
		result = append(result, ProblemTagRecord{Type: "algorithm", Value: "algorithm", Confidence: 0.60})
	}
	return result
}

func buildSolutionText(solution leetcode.Solution) string {
	var b strings.Builder
	if strings.TrimSpace(solution.ContentMarkdown) != "" {
		b.WriteString(strings.TrimSpace(solution.ContentMarkdown))
	}
	for _, lang := range []string{"c", "cpp"} {
		code := strings.TrimSpace(solution.CodeByLanguage[lang])
		if code == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("```")
		b.WriteString(lang)
		b.WriteString("\n")
		b.WriteString(code)
		b.WriteString("\n```")
	}
	return strings.TrimSpace(b.String())
}

func numericQuestionID(raw string) *int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &value
}

func estimatedMinutes(difficulty string) int {
	switch leetcode.NormalizeDifficulty(difficulty) {
	case "Easy":
		return 20
	case "Hard":
		return 50
	case "Medium":
		return 35
	default:
		return 30
	}
}

func qualityScore(problem leetcode.Problem) float64 {
	score := 0.70
	if strings.TrimSpace(problem.ContentMarkdown) != "" {
		score += 0.10
	}
	if strings.TrimSpace(problem.Solution.ContentMarkdown) != "" {
		score += 0.10
	}
	for _, lang := range []string{"c", "cpp"} {
		if strings.TrimSpace(problem.Solution.CodeByLanguage[lang]) != "" {
			score += 0.05
		}
	}
	if score > 1 {
		return 1
	}
	return score
}

func tagCategory(tag string) string {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "array", "linked-list", "linked_list", "stack", "queue", "tree", "binary-tree", "heap", "hash-table", "string", "graph":
		return "data_structure"
	case "two-pointers", "two_pointers", "sliding-window", "sliding_window", "dynamic-programming", "dynamic_programming", "bit-manipulation", "bit_manipulation", "math", "simulation", "prefix-sum", "monotonic-stack", "union-find", "trie":
		return "technique"
	default:
		return "algorithm"
	}
}

func problemURL(slug string) string {
	return fmt.Sprintf("https://leetcode.cn/problems/%s/", strings.TrimSpace(slug))
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func nullInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
