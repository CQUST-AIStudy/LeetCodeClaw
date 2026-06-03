package config

import (
	"os"
	"path/filepath"
	"testing"
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
