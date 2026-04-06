package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/opscompanion/opc/internal/agent"
	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"golang.org/x/term"
)

type configMode string

const (
	configModeKeep      configMode = "keep"
	configModeUpdate    configMode = "update"
	configModeOverwrite configMode = "overwrite"
)

type packageRunner struct {
	Command string
	Args    []string
	Display string
}

type setupPlan struct {
	ConfigMode             configMode
	Agents                 []agent.Info
	APIKey                 string
	APIURL                 string
	InstallCodexRepoSkills bool
	CodexRepoRoot          string
	CodexRunner            packageRunner
}

type agentSetupResult struct {
	Agent      agent.Info
	HooksPath  string
	ConfigPath string
}

type setupResult struct {
	ConfigPath             string
	ConfigVerified         bool
	ConfigOwner            string
	ConfigOrg              string
	ConfigUser             string
	ConfigScopes           []string
	SkillsDir              string
	CodexRulesPath         string
	CodexRulesAdded        bool
	CodexRepoSkillsCommand string
	AgentResults           []agentSetupResult
}

type agentInstallResult struct {
	CodexRulesPath  string
	CodexRulesAdded bool
	CodexHooksPath  string
	CodexConfigPath string
	RepoSkillsCmd   string
}

func isInteractiveSession() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func ensureSkillsRepo() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	installDir := filepath.Join(home, ".opscompanion", "skills")

	if isGitRepo(installDir) {
		fmt.Println("  Updating skills...")
		if err := gitCmd(installDir, "pull", "--quiet"); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: git pull failed: %v\n", err)
		}
		return installDir, nil
	}

	fmt.Println("  Downloading skills...")
	_ = os.RemoveAll(installDir)
	if err := os.MkdirAll(filepath.Dir(installDir), 0o755); err != nil {
		return "", fmt.Errorf("creating skills directory: %w", err)
	}
	if err := gitClone(skillsRepo, installDir); err != nil {
		return "", fmt.Errorf("cloning skills repo: %w", err)
	}
	return installDir, nil
}

func ensureInstallConfig() (*models.Config, string, error) {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &models.Config{
			APIURL: config.DefaultAPIURL,
			APIKey: "mock-key",
		}
		if err := config.Save(cfg); err != nil {
			return nil, "", fmt.Errorf("writing mock config: %w", err)
		}
		fmt.Println("  config: mock mode (run `opc init` or `opc setup` to configure)")
	} else {
		p, _ := config.Path()
		fmt.Printf("  config: %s\n", p)
	}

	p, _ := config.Path()
	return cfg, p, nil
}

func resolveSetupAPIURL(input string, existing *models.Config) string {
	if strings.TrimSpace(input) != "" {
		return strings.TrimRight(strings.TrimSpace(input), "/")
	}
	if envURL := strings.TrimSpace(os.Getenv("OPSCOMPANION_API_URL")); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}
	if existing != nil && strings.TrimSpace(existing.APIURL) != "" {
		return strings.TrimRight(strings.TrimSpace(existing.APIURL), "/")
	}
	return config.DefaultAPIURL
}

func saveVerifiedConfig(apiKey string, apiURL string) (*models.Config, *models.WhoAmIResponse, string, error) {
	cfg := &models.Config{
		APIKey: strings.TrimSpace(apiKey),
		APIURL: strings.TrimRight(strings.TrimSpace(apiURL), "/"),
	}
	if cfg.APIKey == "" {
		return nil, nil, "", fmt.Errorf("API key is required")
	}
	if cfg.APIURL == "" {
		cfg.APIURL = config.DefaultAPIURL
	}

	client := api.New(cfg)
	whoami, err := client.Verify()
	if err != nil {
		return nil, nil, "", fmt.Errorf("verifying API key with %s: %w", cfg.APIURL, err)
	}
	if err := config.Save(cfg); err != nil {
		return nil, nil, "", err
	}

	p, _ := config.Path()
	return cfg, whoami, p, nil
}

func verifyAPIKey(apiKey string, apiURL string) error {
	cfg := &models.Config{
		APIKey: strings.TrimSpace(apiKey),
		APIURL: strings.TrimRight(strings.TrimSpace(apiURL), "/"),
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	if cfg.APIURL == "" {
		cfg.APIURL = config.DefaultAPIURL
	}

	client := api.New(cfg)
	if _, err := client.Verify(); err != nil {
		return fmt.Errorf("verifying API key with %s: %w", cfg.APIURL, err)
	}
	return nil
}

func setupAgentOrder() []agent.Info {
	return []agent.Info{
		agent.Resolve(string(agent.Claude)),
		agent.Resolve(string(agent.Codex)),
		agent.Resolve(string(agent.Cursor)),
		agent.Resolve(string(agent.OpenClaw)),
	}
}

func orderedUniqueAgents(agentsIn []agent.Info) []agent.Info {
	if len(agentsIn) == 0 {
		return nil
	}

	selected := make(map[agent.Name]agent.Info, len(agentsIn))
	for _, ag := range agentsIn {
		selected[ag.Name] = ag
	}

	ordered := make([]agent.Info, 0, len(selected))
	for _, ag := range setupAgentOrder() {
		if chosen, ok := selected[ag.Name]; ok {
			ordered = append(ordered, chosen)
			delete(selected, ag.Name)
		}
	}

	if len(selected) == 0 {
		return ordered
	}

	rest := make([]agent.Info, 0, len(selected))
	for _, ag := range selected {
		rest = append(rest, ag)
	}
	sort.Slice(rest, func(i, j int) bool {
		return string(rest[i].Name) < string(rest[j].Name)
	})
	return append(ordered, rest...)
}

func agentDisplayName(ag agent.Info) string {
	switch ag.Name {
	case agent.Claude:
		return "Claude"
	case agent.Codex:
		return "Codex"
	case agent.Cursor:
		return "Cursor"
	case agent.OpenClaw:
		return "OpenClaw"
	default:
		name := string(ag.Name)
		if name == "" {
			return "Unknown"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}

func agentNamesSummary(agentsIn []agent.Info) string {
	ordered := orderedUniqueAgents(agentsIn)
	names := make([]string, 0, len(ordered))
	for _, ag := range ordered {
		names = append(names, agentDisplayName(ag))
	}
	return strings.Join(names, ", ")
}

func planIncludesAgent(plan setupPlan, want agent.Name) bool {
	for _, ag := range plan.Agents {
		if ag.Name == want {
			return true
		}
	}
	return false
}

func installForAgent(ag agent.Info, installDir string, cfg *models.Config, plan *setupPlan) (agentInstallResult, error) {
	switch ag.Name {
	case agent.Claude:
		return agentInstallResult{}, installClaude(installDir)
	case agent.Codex:
		return installCodex(installDir, cfg, plan)
	default:
		fmt.Printf("  agent %q: no specific installer yet — generating hooks only\n", ag.Name)
		return agentInstallResult{}, nil
	}
}

func generateHooksForAgent(ag agent.Info, cfg *models.Config) (hooksPath string, configPath string, err error) {
	binary, execErr := os.Executable()
	if execErr != nil {
		binary = "opc"
	}

	switch ag.HookFormat {
	case "claude-hooks":
		return generateClaudeHooks(binary, ag, cfg.APIKey)
	case "codex-hooks":
		return generateCodexHooks(binary, ag, cfg.APIKey)
	default:
		return "", "", generateGenericHooks(binary, ag)
	}
}

func ensureSkillsRepoQuiet() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	installDir := filepath.Join(home, ".opscompanion", "skills")

	if isGitRepo(installDir) {
		if err := gitCmdQuiet(installDir, "pull", "--quiet"); err != nil {
			return installDir, nil
		}
		return installDir, nil
	}

	_ = os.RemoveAll(installDir)
	if err := os.MkdirAll(filepath.Dir(installDir), 0o755); err != nil {
		return "", fmt.Errorf("creating skills directory: %w", err)
	}
	if err := gitCloneQuiet(skillsRepo, installDir); err != nil {
		return "", fmt.Errorf("cloning skills repo: %w", err)
	}
	return installDir, nil
}

func installForAgentQuiet(ag agent.Info, installDir string, cfg *models.Config, plan *setupPlan) (agentInstallResult, error) {
	switch ag.Name {
	case agent.Claude:
		return agentInstallResult{}, installClaudeQuiet(installDir)
	case agent.Codex:
		return installCodexQuiet(installDir, cfg, plan)
	default:
		return agentInstallResult{}, nil
	}
}

func generateHooksForAgentQuiet(ag agent.Info, cfg *models.Config) (hooksPath string, configPath string, err error) {
	binary, execErr := os.Executable()
	if execErr != nil {
		binary = "opc"
	}

	switch ag.HookFormat {
	case "claude-hooks":
		return writeClaudeHooks(binary, ag, true, cfg.APIKey)
	case "codex-hooks":
		return writeCodexHooks(binary, ag, true, cfg.APIKey)
	default:
		return "", "", generateGenericHooksQuiet(binary, ag)
	}
}

func findGitRoot(start string) (string, bool) {
	dir := strings.TrimSpace(start)
	if dir == "" {
		return "", false
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}

	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return abs, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

func detectPackageRunner(dir string) packageRunner {
	hasBun := fileExists(filepath.Join(dir, "bun.lock")) || fileExists(filepath.Join(dir, "bun.lockb"))
	hasPNPM := fileExists(filepath.Join(dir, "pnpm-lock.yaml"))
	hasNPM := fileExists(filepath.Join(dir, "package-lock.json")) || fileExists(filepath.Join(dir, "package.json"))

	if hasBun && commandExists("bunx") {
		return packageRunner{Command: "bunx", Display: "bunx"}
	}
	if hasPNPM && commandExists("pnpx") {
		return packageRunner{Command: "pnpx", Display: "pnpx"}
	}
	if hasPNPM && commandExists("pnpm") {
		return packageRunner{Command: "pnpm", Args: []string{"dlx"}, Display: "pnpm dlx"}
	}
	if hasNPM && commandExists("npx") {
		return packageRunner{Command: "npx", Display: "npx"}
	}
	if commandExists("npx") {
		return packageRunner{Command: "npx", Display: "npx"}
	}
	if commandExists("bunx") {
		return packageRunner{Command: "bunx", Display: "bunx"}
	}
	if commandExists("pnpx") {
		return packageRunner{Command: "pnpx", Display: "pnpx"}
	}
	if commandExists("pnpm") {
		return packageRunner{Command: "pnpm", Args: []string{"dlx"}, Display: "pnpm dlx"}
	}

	return packageRunner{Command: "npx", Display: "npx"}
}

func runCodexRepoSkillsInstall(repoRoot string, runner packageRunner) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return "", fmt.Errorf("codex repo-local skills install requires a repository path")
	}
	args := append(append([]string(nil), runner.Args...), "skills", "add", "opscompanion/opscompanion-skills")
	cmd := exec.Command(runner.Command, args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	commandText := strings.TrimSpace(strings.Join(append([]string{runner.Command}, args...), " "))
	if err := cmd.Run(); err != nil {
		return commandText, fmt.Errorf("running %q in %s: %w", commandText, repoRoot, err)
	}
	return commandText, nil
}

func runCodexRepoSkillsInstallQuiet(repoRoot string, runner packageRunner) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return "", fmt.Errorf("codex repo-local skills install requires a repository path")
	}
	args := append(append([]string(nil), runner.Args...), "skills", "add", "opscompanion/opscompanion-skills")
	cmd := exec.Command(runner.Command, args...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	commandText := strings.TrimSpace(strings.Join(append([]string{runner.Command}, args...), " "))
	if err != nil {
		details := strings.TrimSpace(string(output))
		if details != "" {
			return commandText, fmt.Errorf("running %q in %s: %w\n%s", commandText, repoRoot, err, details)
		}
		return commandText, fmt.Errorf("running %q in %s: %w", commandText, repoRoot, err)
	}
	return commandText, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func gitCmdQuiet(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func gitCloneQuiet(repo, dest string) error {
	cmd := exec.Command("git", "clone", "--quiet", repo, dest)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func runShellQuiet(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}
