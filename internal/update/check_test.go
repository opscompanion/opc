package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewerThan(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v0.3.1", "v0.2.1", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.2.2", "v0.2.1", true},
		{"v0.2.1", "v0.2.1", false},
		{"v0.2.0", "v0.2.1", false},
		{"v0.1.0", "v0.2.1", false},
		{"0.3.1", "0.2.1", true},
		{"v1.0.0", "0.9.0", true},
		{"v1.0.0-rc1", "v0.9.0", true},
		{"invalid", "v0.2.1", false},
		{"v0.2.1", "invalid", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.latest+"_vs_"+tt.current, func(t *testing.T) {
			got := newerThan(tt.latest, tt.current)
			if got != tt.want {
				t.Fatalf("newerThan(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"v1.2.3", []int{1, 2, 3}},
		{"0.10.5", []int{0, 10, 5}},
		{"v1.0.0-rc1", []int{1, 0, 0}},
		{"v1.2", nil},
		{"abc", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSemver(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("parseSemver(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseSemver(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("parseSemver(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCacheReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cacheFile)

	entry := cacheEntry{
		Latest:    "v0.3.1",
		CheckedAt: time.Now().Truncate(time.Second),
	}

	if err := writeCache(path, entry); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	got, err := readCache(path)
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if got.Latest != entry.Latest {
		t.Fatalf("Latest = %q, want %q", got.Latest, entry.Latest)
	}
	if !got.CheckedAt.Equal(entry.CheckedAt) {
		t.Fatalf("CheckedAt = %v, want %v", got.CheckedAt, entry.CheckedAt)
	}
}

func TestCacheReadMissing(t *testing.T) {
	_, err := readCache(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing cache file")
	}
}

func TestCheckSkipsDevVersion(t *testing.T) {
	if msg := Check("dev", false); msg != "" {
		t.Fatalf("Check(dev) = %q, want empty", msg)
	}
	if msg := Check("", false); msg != "" {
		t.Fatalf("Check(\"\") = %q, want empty", msg)
	}
}

func TestCheckUsesFreshCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cachePath := filepath.Join(dir, "opscompanion", cacheFile)
	if err := writeCache(cachePath, cacheEntry{
		Latest:    "v1.0.0",
		CheckedAt: time.Now(),
	}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	originalFetcher := latestVersionFetcher
	latestVersionFetcher = func() (string, error) {
		t.Fatal("fetcher should not be called when cache is fresh")
		return "", nil
	}
	t.Cleanup(func() {
		latestVersionFetcher = originalFetcher
	})

	msg := Check("v0.5.0", false)
	if !strings.Contains(msg, "v1.0.0") {
		t.Fatalf("hint should mention cached version, got %q", msg)
	}
}

func TestCheckForceRefreshBypassesCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cachePath := filepath.Join(dir, "opscompanion", cacheFile)
	if err := writeCache(cachePath, cacheEntry{
		Latest:    "v0.4.0",
		CheckedAt: time.Now(),
	}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	originalFetcher := latestVersionFetcher
	latestVersionFetcher = func() (string, error) {
		return "v0.9.0", nil
	}
	t.Cleanup(func() {
		latestVersionFetcher = originalFetcher
	})

	msg := Check("v0.5.0", true)
	if !strings.Contains(msg, "v0.9.0") {
		t.Fatalf("hint should mention refreshed version, got %q", msg)
	}

	got, err := readCache(cachePath)
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if got.Latest != "v0.9.0" {
		t.Fatalf("cache latest = %q, want %q", got.Latest, "v0.9.0")
	}
}

func TestCheckFetchFailureReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	originalFetcher := latestVersionFetcher
	latestVersionFetcher = func() (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() {
		latestVersionFetcher = originalFetcher
	})

	if msg := Check("v0.5.0", true); msg != "" {
		t.Fatalf("Check should suppress fetch failure, got %q", msg)
	}
}

func TestCheckNoUpdateWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cachePath := filepath.Join(dir, "opscompanion", cacheFile)
	if err := writeCache(cachePath, cacheEntry{
		Latest:    "v0.5.0",
		CheckedAt: time.Now(),
	}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	if msg := Check("v0.5.0", false); msg != "" {
		t.Fatalf("expected no hint when current, got %q", msg)
	}
}

func TestConfigDirFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	got := configDir()
	want := filepath.Join(home, ".config", "opscompanion")
	if got != want {
		t.Fatalf("configDir() = %q, want %q", got, want)
	}
}

func TestWriteCacheCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", cacheFile)
	if err := writeCache(path, cacheEntry{Latest: "v1.2.3", CheckedAt: time.Now()}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}
}
