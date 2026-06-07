package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDotEnvSetsUnsetValues(t *testing.T) {
	clearEnv(t, "LEETCODE_CLAW_TEST_VALUE")

	path := writeDotEnv(t, `
# comments and blank lines are ignored
LEETCODE_CLAW_TEST_VALUE=from-file
`)

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if got := os.Getenv("LEETCODE_CLAW_TEST_VALUE"); got != "from-file" {
		t.Fatalf("LEETCODE_CLAW_TEST_VALUE = %q, want %q", got, "from-file")
	}
}

func TestLoadDotEnvKeepsExistingEnv(t *testing.T) {
	t.Setenv("LEETCODE_CLAW_TEST_EXISTING", "from-system")

	path := writeDotEnv(t, `LEETCODE_CLAW_TEST_EXISTING=from-file`)

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if got := os.Getenv("LEETCODE_CLAW_TEST_EXISTING"); got != "from-system" {
		t.Fatalf("LEETCODE_CLAW_TEST_EXISTING = %q, want %q", got, "from-system")
	}
}

func TestLoadDotEnvSupportsExportAndQuotedValues(t *testing.T) {
	clearEnv(t, "LEETCODE_CLAW_TEST_DOUBLE")
	clearEnv(t, "LEETCODE_CLAW_TEST_SINGLE")
	clearEnv(t, "LEETCODE_CLAW_TEST_EXPORT")

	path := writeDotEnv(t, `
export LEETCODE_CLAW_TEST_EXPORT=enabled
LEETCODE_CLAW_TEST_DOUBLE="hello\nworld"
LEETCODE_CLAW_TEST_SINGLE='hello world'
`)

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if got := os.Getenv("LEETCODE_CLAW_TEST_EXPORT"); got != "enabled" {
		t.Fatalf("LEETCODE_CLAW_TEST_EXPORT = %q, want %q", got, "enabled")
	}
	if got := os.Getenv("LEETCODE_CLAW_TEST_DOUBLE"); got != "hello\nworld" {
		t.Fatalf("LEETCODE_CLAW_TEST_DOUBLE = %q, want quoted newline value", got)
	}
	if got := os.Getenv("LEETCODE_CLAW_TEST_SINGLE"); got != "hello world" {
		t.Fatalf("LEETCODE_CLAW_TEST_SINGLE = %q, want %q", got, "hello world")
	}
}

func TestLoadDotEnvMissingFileIsAllowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv() error = %v, want nil", err)
	}
}

func TestLoadReadsDotEnvFromWorkingDirectory(t *testing.T) {
	clearEnv(t, "LEETCODE_CLAW_ADDR")
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("LEETCODE_CLAW_ADDR=:10171\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != ":10171" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, ":10171")
	}
}

func TestLoadUsesMainBackendDatabasePortByDefault(t *testing.T) {
	clearEnv(t, "DB_PORT")

	cfg := loadFromEnv()
	if cfg.Database.Port != "3307" {
		t.Fatalf("Database.Port = %q, want %q", cfg.Database.Port, "3307")
	}
}

func TestLoadUsesCrawlAllDefaults(t *testing.T) {
	clearEnv(t, "LEETCODE_CLAW_CRAWL_ALL_WORKERS")
	clearEnv(t, "LEETCODE_CLAW_CRAWL_ALL_PAGE_SIZE")
	clearEnv(t, "LEETCODE_CLAW_CRAWL_ALL_DELAY")

	cfg := loadFromEnv()

	if cfg.CrawlAll.Workers != 1 {
		t.Fatalf("Workers = %d, want 1", cfg.CrawlAll.Workers)
	}
	if cfg.CrawlAll.PageSize != 100 {
		t.Fatalf("PageSize = %d, want 100", cfg.CrawlAll.PageSize)
	}
	if cfg.CrawlAll.Delay != 2*time.Second {
		t.Fatalf("Delay = %s, want 2s", cfg.CrawlAll.Delay)
	}
}

func TestLoadUsesSecurityDefaults(t *testing.T) {
	clearEnv(t, "LEETCODE_CLAW_API_KEY")
	clearEnv(t, "LEETCODE_CLAW_CORS_ORIGINS")

	cfg := loadFromEnv()

	if cfg.Security.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty", cfg.Security.APIKey)
	}
	if len(cfg.Security.CORSOrigins) != 1 || cfg.Security.CORSOrigins[0] != "*" {
		t.Fatalf("CORSOrigins = %#v, want [*]", cfg.Security.CORSOrigins)
	}
}

func TestLoadReadsSecurityEnv(t *testing.T) {
	t.Setenv("LEETCODE_CLAW_API_KEY", " secret-token ")
	t.Setenv("LEETCODE_CLAW_CORS_ORIGINS", "https://app.example.com, https://admin.example.com, ")

	cfg := loadFromEnv()

	if cfg.Security.APIKey != "secret-token" {
		t.Fatalf("APIKey = %q, want trimmed token", cfg.Security.APIKey)
	}
	want := []string{"https://app.example.com", "https://admin.example.com"}
	if len(cfg.Security.CORSOrigins) != len(want) {
		t.Fatalf("CORSOrigins = %#v, want %#v", cfg.Security.CORSOrigins, want)
	}
	for i := range want {
		if cfg.Security.CORSOrigins[i] != want[i] {
			t.Fatalf("CORSOrigins[%d] = %q, want %q", i, cfg.Security.CORSOrigins[i], want[i])
		}
	}
}

func TestLoadReadsCrawlAllEnv(t *testing.T) {
	t.Setenv("LEETCODE_CLAW_CRAWL_ALL_WORKERS", "3")
	t.Setenv("LEETCODE_CLAW_CRAWL_ALL_PAGE_SIZE", "500")
	t.Setenv("LEETCODE_CLAW_CRAWL_ALL_DELAY", "1500ms")

	cfg := loadFromEnv()

	if cfg.CrawlAll.Workers != 3 {
		t.Fatalf("Workers = %d, want 3", cfg.CrawlAll.Workers)
	}
	if cfg.CrawlAll.PageSize != 200 {
		t.Fatalf("PageSize = %d, want capped 200", cfg.CrawlAll.PageSize)
	}
	if cfg.CrawlAll.Delay != 1500*time.Millisecond {
		t.Fatalf("Delay = %s, want 1500ms", cfg.CrawlAll.Delay)
	}
}

func TestLoadDotEnvRejectsMalformedLines(t *testing.T) {
	path := writeDotEnv(t, `LEETCODE_CLAW_BAD_LINE`)

	if err := loadDotEnv(path); err == nil {
		t.Fatal("loadDotEnv() error = nil, want malformed line error")
	}
}

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return path
}

func clearEnv(t *testing.T, key string) {
	t.Helper()
	oldValue, hadOldValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q): %v", key, err)
	}
	t.Cleanup(func() {
		if hadOldValue {
			if err := os.Setenv(key, oldValue); err != nil {
				t.Fatalf("restore env %q: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("cleanup env %q: %v", key, err)
		}
	})
}
