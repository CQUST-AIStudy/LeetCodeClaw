package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	CrawlJobStatusQueued        = "queued"
	CrawlJobStatusRunning       = "running"
	CrawlJobStatusSucceeded     = "succeeded"
	CrawlJobStatusPartialFailed = "partial_failed"
	CrawlJobStatusFailed        = "failed"
	CrawlJobStatusCanceled      = "canceled"

	CrawlJobItemStatusPending   = "pending"
	CrawlJobItemStatusRunning   = "running"
	CrawlJobItemStatusSucceeded = "succeeded"
	CrawlJobItemStatusFailed    = "failed"
)

type CrawlJobConfig struct {
	Persist      bool
	ForceRefresh bool
	Workers      int
	Delay        time.Duration
	PageSize     int
}

type CrawlJob struct {
	ID           int64
	Status       string
	Persist      bool
	ForceRefresh bool
	Workers      int
	Delay        time.Duration
	PageSize     int
	Total        int
	Succeeded    int
	Failed       int
	Error        string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type CrawlJobItem struct {
	ID         int64
	JobID      int64
	Slug       string
	Status     string
	Error      string
	ProblemID  int64
	Attempts   int
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

type CrawlJobDetail struct {
	Job         CrawlJob
	FailedItems []CrawlJobItem
}

func (s *Store) EnsureCrawlJobSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS leetcode_crawl_job (
  id BIGINT NOT NULL AUTO_INCREMENT,
  status VARCHAR(32) NOT NULL,
  persist TINYINT(1) NOT NULL DEFAULT 1,
  force_refresh TINYINT(1) NOT NULL DEFAULT 1,
  workers INT NOT NULL DEFAULT 1,
  delay_millis INT NOT NULL DEFAULT 2000,
  page_size INT NOT NULL DEFAULT 100,
  total_count INT NOT NULL DEFAULT 0,
  success_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  error_text TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMP NULL,
  finished_at TIMESTAMP NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_leetcode_crawl_job_status (status),
  KEY idx_leetcode_crawl_job_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS leetcode_crawl_job_item (
  id BIGINT NOT NULL AUTO_INCREMENT,
  job_id BIGINT NOT NULL,
  slug VARCHAR(191) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  error_text TEXT NULL,
  problem_id BIGINT NULL,
  attempts INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMP NULL,
  finished_at TIMESTAMP NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_leetcode_crawl_job_item (job_id, slug),
  KEY idx_leetcode_crawl_job_item_job_status (job_id, status),
  CONSTRAINT fk_leetcode_crawl_job_item_job
    FOREIGN KEY (job_id) REFERENCES leetcode_crawl_job(id)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	return nil
}

func (s *Store) FindActiveCrawlJob(ctx context.Context) (*CrawlJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT id, status, persist, force_refresh, workers, delay_millis, page_size,
       total_count, success_count, failed_count, error_text, created_at, started_at, finished_at
FROM leetcode_crawl_job
WHERE status IN (?, ?)
ORDER BY id DESC
LIMIT 1`, CrawlJobStatusQueued, CrawlJobStatusRunning)
	job, err := scanCrawlJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) CreateCrawlJob(ctx context.Context, cfg CrawlJobConfig) (CrawlJob, error) {
	if s == nil || s.db == nil {
		return CrawlJob{}, errors.New("store is not initialized")
	}
	cfg = normalizeCrawlJobConfig(cfg)
	result, err := s.db.ExecContext(ctx, `
INSERT INTO leetcode_crawl_job (
  status, persist, force_refresh, workers, delay_millis, page_size
) VALUES (?, ?, ?, ?, ?, ?)`,
		CrawlJobStatusQueued,
		cfg.Persist,
		cfg.ForceRefresh,
		cfg.Workers,
		int(cfg.Delay/time.Millisecond),
		cfg.PageSize,
	)
	if err != nil {
		return CrawlJob{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return CrawlJob{}, err
	}
	job, err := s.GetCrawlJob(ctx, id)
	if err != nil {
		return CrawlJob{}, err
	}
	if job == nil {
		return CrawlJob{}, errors.New("created crawl job was not found")
	}
	return *job, nil
}

func (s *Store) MarkCrawlJobRunning(ctx context.Context, jobID int64, total int) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	if total < 0 {
		total = 0
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job
SET status = ?, total_count = ?, started_at = COALESCE(started_at, CURRENT_TIMESTAMP)
WHERE id = ?`, CrawlJobStatusRunning, total, jobID)
	return err
}

func (s *Store) AddCrawlJobItems(ctx context.Context, jobID int64, slugs []string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, slug := range normalizeJobSlugs(slugs) {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO leetcode_crawl_job_item (job_id, slug, status)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE slug = VALUES(slug)`, jobID, slug, CrawlJobItemStatusPending); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListCrawlJobItemsByStatus(ctx context.Context, jobID int64, status string) ([]CrawlJobItem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, job_id, slug, status, error_text, problem_id, attempts, created_at, started_at, finished_at
FROM leetcode_crawl_job_item
WHERE job_id = ? AND status = ?
ORDER BY id ASC`, jobID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCrawlJobItems(rows)
}

func (s *Store) MarkCrawlJobItemRunning(ctx context.Context, itemID int64) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job_item
SET status = ?, attempts = attempts + 1, started_at = COALESCE(started_at, CURRENT_TIMESTAMP), error_text = NULL
WHERE id = ?`, CrawlJobItemStatusRunning, itemID)
	return err
}

func (s *Store) MarkCrawlJobItemSucceeded(ctx context.Context, itemID int64, problemID int64) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	var nullableProblemID sql.NullInt64
	if problemID > 0 {
		nullableProblemID = sql.NullInt64{Int64: problemID, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job_item
SET status = ?, problem_id = ?, error_text = NULL, finished_at = CURRENT_TIMESTAMP
WHERE id = ?`, CrawlJobItemStatusSucceeded, nullableProblemID, itemID)
	return err
}

func (s *Store) MarkCrawlJobItemFailed(ctx context.Context, itemID int64, message string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job_item
SET status = ?, error_text = ?, finished_at = CURRENT_TIMESTAMP
WHERE id = ?`, CrawlJobItemStatusFailed, trimPersistedError(message), itemID)
	return err
}

func (s *Store) FinishCrawlJob(ctx context.Context, jobID int64) (CrawlJob, error) {
	if s == nil || s.db == nil {
		return CrawlJob{}, errors.New("store is not initialized")
	}
	var total, succeeded, failed, unfinished int
	if err := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*),
  COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status NOT IN (?, ?) THEN 1 ELSE 0 END), 0)
FROM leetcode_crawl_job_item
WHERE job_id = ?`,
		CrawlJobItemStatusSucceeded,
		CrawlJobItemStatusFailed,
		CrawlJobItemStatusSucceeded,
		CrawlJobItemStatusFailed,
		jobID,
	).Scan(&total, &succeeded, &failed, &unfinished); err != nil {
		return CrawlJob{}, err
	}

	if unfinished > 0 {
		failed += unfinished
	}

	status := CrawlJobStatusSucceeded
	switch {
	case unfinished > 0:
		status = CrawlJobStatusFailed
	case failed > 0 && succeeded > 0:
		status = CrawlJobStatusPartialFailed
	case failed > 0:
		status = CrawlJobStatusFailed
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job
SET status = ?, total_count = ?, success_count = ?, failed_count = ?, finished_at = CURRENT_TIMESTAMP
WHERE id = ?`, status, total, succeeded, failed, jobID); err != nil {
		return CrawlJob{}, err
	}
	job, err := s.GetCrawlJob(ctx, jobID)
	if err != nil {
		return CrawlJob{}, err
	}
	if job == nil {
		return CrawlJob{}, errors.New("finished crawl job was not found")
	}
	return *job, nil
}

func (s *Store) FailCrawlJob(ctx context.Context, jobID int64, message string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job
SET status = ?, error_text = ?, finished_at = CURRENT_TIMESTAMP
WHERE id = ?`, CrawlJobStatusFailed, trimPersistedError(message), jobID)
	return err
}

func (s *Store) CancelActiveCrawlJobs(ctx context.Context, message string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE leetcode_crawl_job
SET status = ?, error_text = ?, finished_at = CURRENT_TIMESTAMP
WHERE status IN (?, ?)`,
		CrawlJobStatusCanceled,
		trimPersistedError(message),
		CrawlJobStatusQueued,
		CrawlJobStatusRunning,
	)
	return err
}

func (s *Store) GetCrawlJob(ctx context.Context, jobID int64) (*CrawlJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT id, status, persist, force_refresh, workers, delay_millis, page_size,
       total_count, success_count, failed_count, error_text, created_at, started_at, finished_at
FROM leetcode_crawl_job
WHERE id = ?`, jobID)
	job, err := scanCrawlJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (s *Store) GetCrawlJobDetail(ctx context.Context, jobID int64) (*CrawlJobDetail, error) {
	job, err := s.GetCrawlJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	total, succeeded, failed, err := s.crawlJobProgressCounts(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if total > 0 || job.Status == CrawlJobStatusRunning || job.Status == CrawlJobStatusQueued {
		job.Total = total
		job.Succeeded = succeeded
		job.Failed = failed
	}
	failedItems, err := s.ListCrawlJobItemsByStatus(ctx, jobID, CrawlJobItemStatusFailed)
	if err != nil {
		return nil, err
	}
	return &CrawlJobDetail{Job: *job, FailedItems: failedItems}, nil
}

func (s *Store) crawlJobProgressCounts(ctx context.Context, jobID int64) (int, int, int, error) {
	var total, succeeded, failed int
	if err := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*),
  COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0)
FROM leetcode_crawl_job_item
WHERE job_id = ?`, CrawlJobItemStatusSucceeded, CrawlJobItemStatusFailed, jobID).Scan(&total, &succeeded, &failed); err != nil {
		return 0, 0, 0, err
	}
	return total, succeeded, failed, nil
}

type crawlJobScanner interface {
	Scan(dest ...any) error
}

func scanCrawlJob(scanner crawlJobScanner) (*CrawlJob, error) {
	var job CrawlJob
	var delayMillis int
	var errorText sql.NullString
	var startedAt, finishedAt sql.NullTime
	if err := scanner.Scan(
		&job.ID,
		&job.Status,
		&job.Persist,
		&job.ForceRefresh,
		&job.Workers,
		&delayMillis,
		&job.PageSize,
		&job.Total,
		&job.Succeeded,
		&job.Failed,
		&errorText,
		&job.CreatedAt,
		&startedAt,
		&finishedAt,
	); err != nil {
		return nil, err
	}
	job.Delay = time.Duration(delayMillis) * time.Millisecond
	job.Error = errorText.String
	job.StartedAt = nullableTimePtr(startedAt)
	job.FinishedAt = nullableTimePtr(finishedAt)
	return &job, nil
}

func scanCrawlJobItems(rows *sql.Rows) ([]CrawlJobItem, error) {
	items := []CrawlJobItem{}
	for rows.Next() {
		var item CrawlJobItem
		var errorText sql.NullString
		var problemID sql.NullInt64
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.JobID,
			&item.Slug,
			&item.Status,
			&errorText,
			&problemID,
			&item.Attempts,
			&item.CreatedAt,
			&startedAt,
			&finishedAt,
		); err != nil {
			return nil, err
		}
		item.Error = errorText.String
		if problemID.Valid {
			item.ProblemID = problemID.Int64
		}
		item.StartedAt = nullableTimePtr(startedAt)
		item.FinishedAt = nullableTimePtr(finishedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullableTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func normalizeCrawlJobConfig(cfg CrawlJobConfig) CrawlJobConfig {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = 100
	}
	if cfg.PageSize > 200 {
		cfg.PageSize = 200
	}
	if cfg.Delay < 0 {
		cfg.Delay = 0
	}
	return cfg
}

func normalizeJobSlugs(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		slug := strings.TrimSpace(value)
		if slug == "" {
			continue
		}
		key := strings.ToLower(slug)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, slug)
	}
	return result
}

func trimPersistedError(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	if len(runes) > 2000 {
		return fmt.Sprintf("%s...", string(runes[:2000]))
	}
	return message
}
