package config

// TOMLConfig represents the full config.toml file structure.
type TOMLConfig struct {
	Default  ProfileEntry            `toml:"default"`
	Profiles map[string]ProfileEntry `toml:"profile"`
}

// ProfileEntry represents one profile (or the default section).
type ProfileEntry struct {
	APIURL string   `toml:"api_url,omitempty"`
	APIKey string   `toml:"api_key"`
	Paths  []string `toml:"paths,omitempty"`
}
