package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cacheFile    = "version-check.json"
	cacheTTL     = 24 * time.Hour
	fetchTimeout = 3 * time.Second
	releasesURL  = "https://api.github.com/repos/opscompanion/opc/releases/latest"
)

var latestVersionFetcher = fetchLatest

type cacheEntry struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// Check returns an update hint message if a newer version is available, or "".
// It reads from a 24h cache and falls back to a GitHub API fetch with a 3s timeout.
// forceRefresh bypasses cache freshness (used by `opc version`).
func Check(currentVersion string, forceRefresh bool) string {
	if currentVersion == "" || currentVersion == "dev" {
		return ""
	}

	cacheDir := configDir()
	cachePath := filepath.Join(cacheDir, cacheFile)

	var latest string

	if !forceRefresh {
		if cached, err := readCache(cachePath); err == nil {
			if time.Since(cached.CheckedAt) < cacheTTL {
				latest = cached.Latest
			}
		}
	}

	if latest == "" {
		var err error
		latest, err = latestVersionFetcher()
		if err != nil {
			return ""
		}
		_ = writeCache(cachePath, cacheEntry{Latest: latest, CheckedAt: time.Now()})
	}

	if newerThan(latest, currentVersion) {
		return fmt.Sprintf("opc: update available %s → %s — run `brew upgrade opc`", currentVersion, latest)
	}
	return ""
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opscompanion")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opscompanion")
}

func readCache(path string) (cacheEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheEntry{}, err
	}
	var c cacheEntry
	if err := json.Unmarshal(data, &c); err != nil {
		return cacheEntry{}, err
	}
	return c, nil
}

func writeCache(path string, c cacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fetchLatest() (string, error) {
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Get(releasesURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", err
	}

	if rel.TagName == "" {
		return "", fmt.Errorf("github: empty tag_name")
	}
	return rel.TagName, nil
}

// newerThan returns true if latest is a higher semver than current.
// Both may optionally have a "v" prefix.
func newerThan(latest, current string) bool {
	l := parseSemver(latest)
	c := parseSemver(current)
	if l == nil || c == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		// Strip pre-release suffix (e.g. "1-rc1" → "1")
		if idx := strings.IndexAny(p, "-+"); idx != -1 {
			p = p[:idx]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}
