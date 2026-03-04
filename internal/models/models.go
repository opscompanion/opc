package models

import "time"

// Config holds the CLI configuration.
type Config struct {
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
	Org    string `json:"org"`
}

// Session represents an agent session.
type Session struct {
	ID          string    `json:"session_id"`
	Org         string    `json:"org"`
	User        string    `json:"user"`
	Started     time.Time `json:"started"`
	Ended       time.Time `json:"ended,omitempty"`
	Repo        string    `json:"repo"`
	Branch      string    `json:"branch"`
	RecentFiles []string  `json:"recent_files,omitempty"`
}

// OrgContext holds org-level configuration.
type OrgContext struct {
	Name            string `json:"name"`
	CloudProvider   string `json:"cloud_provider"`
	IaC             string `json:"iac"`
	CICD            string `json:"ci_cd"`
	Observability   string `json:"observability"`
	SecretsManager  string `json:"secrets_manager"`
	IncidentProcess string `json:"incident_process"`
}

// TeamContext holds team-level configuration.
type TeamContext struct {
	Name              string   `json:"name"`
	Services          []string `json:"services"`
	OnCallRotation    string   `json:"on_call_rotation"`
	DeploymentCadence string   `json:"deployment_cadence"`
	ActiveProjects    []string `json:"active_projects"`
}

// UserContext holds user-level configuration.
type UserContext struct {
	Name       string   `json:"name"`
	Role       string   `json:"role"`
	Prefs      string   `json:"preferences"`
	RecentWork []string `json:"recent_work"`
	Editor     string   `json:"editor"`
	Shell      string   `json:"shell"`
}

// EnvContext holds environment metadata.
type EnvContext struct {
	Branch     string `json:"current_branch"`
	WorkDir    string `json:"working_directory"`
	LastCommit string `json:"last_commit,omitempty"`
}

// FullContext is the combined org/team/user/env context.
type FullContext struct {
	Org  OrgContext  `json:"org"`
	Team TeamContext `json:"team"`
	User UserContext `json:"user"`
	Env  EnvContext  `json:"environment"`
}

// Memory represents a stored decision, discovery, or context entry.
type Memory struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"` // Decision, Discovery, Context
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	Org       string   `json:"org"`
	User      string   `json:"user"`
	CreatedAt string   `json:"created_at"`
	SessionID string   `json:"session_id,omitempty"`
	Relevance float64  `json:"relevance,omitempty"`
}

// SearchResult wraps memory search responses.
type SearchResult struct {
	Results      []Memory `json:"results"`
	TotalMatches int      `json:"total_matches"`
	QueryTimeMS  int      `json:"query_time_ms"`
}

// CommitRecord links a git commit to a session.
type CommitRecord struct {
	SessionID string `json:"session_id"`
	Hash      string `json:"commit_hash"`
	Short     string `json:"commit_short"`
	Message   string `json:"commit_message"`
	Branch    string `json:"branch"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
}

// HistoryEntry combines a commit with its session info.
type HistoryEntry struct {
	Commit    CommitRecord `json:"commit"`
	Agent     string       `json:"agent"`
	SessionID string       `json:"session_id"`
	Decisions []string     `json:"decisions,omitempty"`
}

// Checkpoint represents a session checkpoint.
type Checkpoint struct {
	SessionID     string   `json:"session_id"`
	Compressed    string   `json:"compressed_summary"`
	Decisions     []string `json:"decisions"`
	FilesModified []string `json:"files_modified"`
}

// SessionResumeContext is returned when resuming a session.
type SessionResumeContext struct {
	LastActive    string   `json:"last_active"`
	WorkingOn     string   `json:"working_on"`
	Branch        string   `json:"branch"`
	ModifiedFiles []string `json:"modified_files"`
	Decisions     []string `json:"decisions"`
	OpenThreads   []string `json:"open_threads"`
}
