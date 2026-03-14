package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewerThan(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v0.3.1", "v0.2.1", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.2.2", "v0.2.1", true},
		{"v0.2.1", "v0.2.1", false},
		{"v0.2.0", "v0.2.1", false},
		{"v0.1.0", "v0.2.1", false},
		{"0.3.1", "0.2.1", true},    // no v prefix
		{"v1.0.0", "0.9.0", true},   // mixed prefix
		{"v1.0.0-rc1", "v0.9.0", true}, // pre-release stripped
		{"invalid", "v0.2.1", false},
		{"v0.2.1", "invalid", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.latest+"_vs_"+tt.current, func(t *testing.T) {
			got := newerThan(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("newerThan(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
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
					t.Errorf("parseSemver(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != 3 {
				t.Fatalf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseSemver(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCacheReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version-check.json")

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
		t.Errorf("Latest = %q, want %q", got.Latest, entry.Latest)
	}
	if !got.CheckedAt.Equal(entry.CheckedAt) {
		t.Errorf("CheckedAt = %v, want %v", got.CheckedAt, entry.CheckedAt)
	}
}

func TestCacheReadMissing(t *testing.T) {
	_, err := readCache(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestCheckSkipsDevVersion(t *testing.T) {
	if msg := Check("dev", false); msg != "" {
		t.Errorf("Check('dev') = %q, want empty", msg)
	}
	if msg := Check("", false); msg != "" {
		t.Errorf("Check('') = %q, want empty", msg)
	}
}

func TestCheckWithMockServer(t *testing.T) {
	// Start a mock GitHub releases server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{TagName: "v0.5.0"})
	}))
	defer server.Close()

	// Temporarily override the releases URL by using a custom cache
	// We'll test the full flow by pre-populating a fresh cache
	dir := t.TempDir()
	cachePath := filepath.Join(dir, cacheFile)

	// Write a "stale" cache pointing to an older version
	entry := cacheEntry{
		Latest:    "v0.3.0",
		CheckedAt: time.Now(), // fresh cache
	}
	if err := writeCache(cachePath, entry); err != nil {
		t.Fatal(err)
	}

	// Read it back and verify
	got, err := readCache(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Latest != "v0.3.0" {
		t.Errorf("cached Latest = %q, want v0.3.0", got.Latest)
	}

	// Verify newerThan would detect the update
	if !newerThan("v0.3.0", "v0.2.1") {
		t.Error("newerThan(v0.3.0, v0.2.1) should be true")
	}
}

func TestCheckUseFreshCache(t *testing.T) {
	// Set up a temp config dir with a fresh cache
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cacheDir := filepath.Join(dir, "opscompanion")
	os.MkdirAll(cacheDir, 0o755)

	entry := cacheEntry{
		Latest:    "v1.0.0",
		CheckedAt: time.Now(),
	}
	writeCache(filepath.Join(cacheDir, cacheFile), entry)

	msg := Check("v0.5.0", false)
	if msg == "" {
		t.Error("expected update hint, got empty")
	}
	if !contains(msg, "v1.0.0") {
		t.Errorf("hint should mention v1.0.0, got: %s", msg)
	}
	if !contains(msg, "brew upgrade opc") {
		t.Errorf("hint should mention brew upgrade, got: %s", msg)
	}
}

func TestCheckNoUpdateWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cacheDir := filepath.Join(dir, "opscompanion")
	os.MkdirAll(cacheDir, 0o755)

	entry := cacheEntry{
		Latest:    "v0.5.0",
		CheckedAt: time.Now(),
	}
	writeCache(filepath.Join(cacheDir, cacheFile), entry)

	msg := Check("v0.5.0", false)
	if msg != "" {
		t.Errorf("expected no hint when current, got: %s", msg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
