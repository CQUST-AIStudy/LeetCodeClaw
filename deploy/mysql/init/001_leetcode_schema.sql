SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS leetcode_problem_bank (
  id BIGINT NOT NULL AUTO_INCREMENT,
  source_key VARCHAR(191) NOT NULL,
  problem_code VARCHAR(64) NULL,
  numeric_id INT NULL,
  title_main VARCHAR(512) NOT NULL,
  title_alt VARCHAR(512) NULL,
  problem_text LONGTEXT NOT NULL,
  solution_text LONGTEXT NOT NULL,
  code_snippets_json LONGTEXT NULL,
  source_url VARCHAR(1024) NULL,
  difficulty VARCHAR(32) NOT NULL,
  estimated_minutes INT NOT NULL DEFAULT 30,
  quality_score DECIMAL(5, 4) NOT NULL DEFAULT 0.7000,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_leetcode_problem_bank_source_key (source_key),
  KEY idx_leetcode_problem_bank_numeric_id (numeric_id),
  KEY idx_leetcode_problem_bank_difficulty (difficulty)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS leetcode_problem_tag (
  id BIGINT NOT NULL AUTO_INCREMENT,
  problem_id BIGINT NOT NULL,
  tag_name VARCHAR(191) NOT NULL,
  tag_category VARCHAR(64) NOT NULL,
  relevance_score DECIMAL(5, 4) NOT NULL DEFAULT 0.8000,
  is_primary TINYINT(1) NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_leetcode_problem_tag (problem_id, tag_name, tag_category),
  KEY idx_leetcode_problem_tag_problem_id (problem_id),
  KEY idx_leetcode_problem_tag_name (tag_name),
  CONSTRAINT fk_leetcode_problem_tag_problem
    FOREIGN KEY (problem_id) REFERENCES leetcode_problem_bank(id)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
