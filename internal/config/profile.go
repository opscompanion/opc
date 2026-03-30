package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/opscompanion/opc/internal/models"
)

// activeProfile is set by SetProfile (from the --profile flag) before any
// command runs. An empty string means "not explicitly set".
var activeProfile string

// SetProfile stores the profile name chosen via the --profile flag.
func SetProfile(name string) {
	activeProfile = name
}

// ResolveProfileName returns the profile to use, following the resolution
// chain: flag → env → dotfile → path glob → default (empty string).
func ResolveProfileName(tc *TOMLConfig) string {
	if activeProfile != "" {
		return activeProfile
	}
	if env := strings.TrimSpace(os.Getenv("OPC_PROFILE")); env != "" {
		return env
	}
	if name := findDotfile(); name != "" {
		return name
	}
	if tc != nil {
		if name := matchPathGlobs(tc.Profiles); name != "" {
			return name
		}
	}
	return ""
}

// findDotfile walks from cwd up to the filesystem root looking for a
// .opscompanion file. If found, it reads the "profile = <name>" line.
func findDotfile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, ".opscompanion"))
		if err == nil {
			if name := parseDotfile(string(data)); name != "" {
				return name
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// parseDotfile extracts the profile name from dotfile content.
// Looks for a line like "profile = acme-corp".
func parseDotfile(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "profile" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

// matchPathGlobs checks cwd against each profile's paths globs.
// Returns the first matching profile name, or "".
func matchPathGlobs(profiles map[string]ProfileEntry) string {
	if len(profiles) == 0 {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()

	for name, p := range profiles {
		for _, pattern := range p.Paths {
			expanded := expandHome(pattern, home)
			if matched, _ := filepath.Match(expanded, cwd); matched {
				return name
			}
			// Also try matching as a prefix (for directories under the pattern).
			if strings.HasSuffix(expanded, "/*") || strings.HasSuffix(expanded, "/**") {
				prefix := strings.TrimRight(expanded, "/*")
				if strings.HasPrefix(cwd, prefix+"/") || cwd == prefix {
					return name
				}
			}
		}
	}
	return ""
}

// expandHome replaces a leading ~ with the home directory path.
func expandHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// resolveProfile takes a TOMLConfig and a profile name, and returns a
// *models.Config. Named profiles inherit api_url from [default] when empty.
func resolveProfile(tc *TOMLConfig, profileName string) *models.Config {
	if profileName == "" || tc.Profiles == nil {
		return &models.Config{
			APIURL: tc.Default.APIURL,
			APIKey: tc.Default.APIKey,
		}
	}
	p, ok := tc.Profiles[profileName]
	if !ok {
		return &models.Config{
			APIURL: tc.Default.APIURL,
			APIKey: tc.Default.APIKey,
		}
	}
	apiURL := p.APIURL
	if apiURL == "" {
		apiURL = tc.Default.APIURL
	}
	return &models.Config{
		APIURL: apiURL,
		APIKey: p.APIKey,
	}
}
