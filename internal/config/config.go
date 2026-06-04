package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Config struct {
	Addr       string
	HTTP       HTTPConfig
	LeetCode   LeetCodeConfig
	Database   DatabaseConfig
	DBRequired bool
}

type HTTPConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type LeetCodeConfig struct {
	Timeout time.Duration
	Retries int
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	Username string
	Password string
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}
	return loadFromEnv(), nil
}

func loadFromEnv() Config {
	return Config{
		Addr: getEnv("LEETCODE_CLAW_ADDR", ":10170"),
		HTTP: HTTPConfig{
			ReadTimeout:  getDurationEnv("LEETCODE_CLAW_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDurationEnv("LEETCODE_CLAW_WRITE_TIMEOUT", 120*time.Second),
		},
		LeetCode: LeetCodeConfig{
			Timeout: getDurationEnv("LEETCODE_CLAW_UPSTREAM_TIMEOUT", 20*time.Second),
			Retries: getIntEnv("LEETCODE_CLAW_RETRIES", 2),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnv("DB_PORT", "3307"),
			Name:     getEnv("DB_NAME", "ptadatabase"),
			Username: getEnv("DB_USERNAME", "root"),
			Password: firstNonEmpty(os.Getenv("DB_PASSWORD"), os.Getenv("DB_PASS")),
		},
		DBRequired: getEnv("LEETCODE_CLAW_DB_REQUIRED", "false") == "true",
	}
}

func (c DatabaseConfig) DSN() string {
	cfg := mysql.Config{
		User:      c.Username,
		Passwd:    c.Password,
		Net:       "tcp",
		Addr:      net.JoinHostPort(c.Host, c.Port),
		DBName:    c.Name,
		ParseTime: true,
		Loc:       time.Local,
		Params: map[string]string{
			"charset": "utf8mb4",
		},
	}
	return cfg.FormatDSN()
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getIntEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d invalid .env line", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if !validEnvKey(key) {
			return fmt.Errorf("%s:%d invalid env key %q", path, lineNo, key)
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value, err := parseEnvValue(strings.TrimSpace(rawValue))
		if err != nil {
			return fmt.Errorf("%s:%d %w", path, lineNo, err)
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func parseEnvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, `"`) {
		if !strings.HasSuffix(raw, `"`) || len(raw) == 1 {
			return "", fmt.Errorf("unterminated double-quoted value")
		}
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", err
		}
		return value, nil
	}
	if strings.HasPrefix(raw, "'") {
		if !strings.HasSuffix(raw, "'") || len(raw) == 1 {
			return "", fmt.Errorf("unterminated single-quoted value")
		}
		return raw[1 : len(raw)-1], nil
	}
	return raw, nil
}
