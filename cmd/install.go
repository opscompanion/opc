package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opscompanion/opc/internal/agent"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

const (
	skillsRepo   = "https://github.com/opscompanion/opscompanion-skills.git"
	codexOPCRule = `prefix_rule(
  pattern = ["opc"],
  decision = "allow",
  justification = "Allow OpsCompanion CLI usage outside the sandbox",
)`
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install OpsCompanion skills and hooks for an agent",
	Long: `Sets up OpsCompanion for the specified agent runtime.

When run in an interactive terminal without --agent, opc prompts for the
target runtime instead of failing immediately.

Examples:

  opc install --agent claude    Install plugin + hooks for Claude Code
  opc install --agent codex     Install skills + hooks for Codex

This command:
  1. Downloads/updates the OpsCompanion skills repository
  2. Ensures a config file exists (mock mode if unconfigured)
  3. Installs agent-specific skills/plugins
  4. Generates hook configuration

It is the Go-native equivalent of the curl installer script.`,
	RunE: runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	ag := ActiveAgent
	if ag.Name == agent.Unknown || ag.Name == agent.Auto {
		if !isInteractiveSession() {
			return fmt.Errorf("--agent is required for install (e.g. opc install --agent claude)")
		}
		selected, err := runAgentSelectModel()
		if err != nil {
			return err
		}
		ag = selected
	}

	fmt.Println()
	fmt.Println("  opscompanion installer")
	fmt.Println()

	// ── 1. Ensure skills repo ──────────────────────────────────────────────

	installDir, err := ensureSkillsRepo()
	if err != nil {
		return err
	}

	// ── 2. Ensure config exists ────────────────────────────────────────────

	cfg, _, err := ensureInstallConfig()
	if err != nil {
		return err
	}

	// ── 3. Agent-specific installation ─────────────────────────────────────

	if _, err := installForAgent(ag, installDir, cfg, nil); err != nil {
		return err
	}

	// ── 4. Generate hooks ──────────────────────────────────────────────────

	fmt.Println()
	fmt.Println("  Generating hooks...")
	if _, _, err := generateHooksForAgent(ag, cfg); err != nil {
		return fmt.Errorf("generating hooks: %w", err)
	}

	fmt.Println()
	fmt.Println("  Restart your agent for changes to take effect.")
	fmt.Println()
	return nil
}

func installClaude(installDir string) error {
	marketplaceURL := "https://github.com/opscompanion/opscompanion-skills"

	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		fmt.Println("  warning: 'claude' not found in PATH — skipping plugin registration")
		fmt.Println("  (install Claude Code first, then re-run `opc install --agent claude`)")
		return nil
	}

	fmt.Printf("  claude: %s\n", claudeBin)

	fmt.Println("  Registering marketplace...")
	runShell(claudeBin, "plugin", "marketplace", "add", marketplaceURL)

	fmt.Println("  Installing plugin...")
	runShell(claudeBin, "plugin", "install", "opscompanion")

	fmt.Println()
	fmt.Println("  Installed for Claude Code.")
	fmt.Println()
	fmt.Println("  Skills:")
	fmt.Println("    /opscompanion-init       Set up OpsCompanion")
	fmt.Println("    /opscompanion-context    Load org/team/user context")
	fmt.Println("    /opscompanion-search     Search team memories")
	fmt.Println("    /opscompanion-remember   Save a decision")
	return nil
}

func installClaudeQuiet(installDir string) error {
	_ = installDir
	marketplaceURL := "https://github.com/opscompanion/opscompanion-skills"

	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		return nil
	}

	runShellQuiet(claudeBin, "plugin", "marketplace", "add", marketplaceURL)
	runShellQuiet(claudeBin, "plugin", "install", "opscompanion")
	return nil
}

func installCodex(installDir string, cfg *models.Config, plan *setupPlan) (agentInstallResult, error) {
	skillsSrc := filepath.Join(installDir, "agents", "skills")
	home, _ := os.UserHomeDir()
	skillsDst := filepath.Join(home, ".agents", "skills")

	if err := os.MkdirAll(skillsDst, 0755); err != nil {
		return agentInstallResult{}, fmt.Errorf("creating skills directory: %w", err)
	}

	skills := []string{
		"opscompanion-init",
		"opscompanion-context",
		"opscompanion-search",
		"opscompanion-remember",
	}

	for _, skill := range skills {
		link := filepath.Join(skillsDst, skill)
		target := filepath.Join(skillsSrc, skill)

		// Remove existing link/dir
		_ = os.RemoveAll(link)

		if err := os.Symlink(target, link); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to symlink %s: %v\n", skill, err)
		}
	}

	rulesPath, added, err := ensureCodexOPCRule(home)
	if err != nil {
		return agentInstallResult{}, err
	}

	result := agentInstallResult{
		CodexRulesPath:  rulesPath,
		CodexRulesAdded: added,
	}

	if plan != nil && plan.InstallCodexRepoSkills {
		fmt.Printf("  Codex repo skills: %s\n", plan.CodexRunner.Display)
		commandText, err := runCodexRepoSkillsInstall(plan.CodexRepoRoot, plan.CodexRunner)
		result.RepoSkillsCmd = commandText
		if err != nil {
			return result, err
		}
	}

	fmt.Println()
	fmt.Println("  Installed for Codex.")
	fmt.Println()
	fmt.Println("  Skills (in ~/.agents/skills/):")
	fmt.Println("    $opscompanion-init       Set up OpsCompanion")
	fmt.Println("    $opscompanion-context    Load org/team/user context")
	fmt.Println("    $opscompanion-search     Search team memories")
	fmt.Println("    $opscompanion-remember   Save a decision")
	if added {
		fmt.Printf("  Rules: added opc prefix rule to %s\n", rulesPath)
	} else {
		fmt.Printf("  Rules: opc prefix rule already present in %s\n", rulesPath)
	}
	_ = cfg
	return result, nil
}

func installCodexQuiet(installDir string, cfg *models.Config, plan *setupPlan) (agentInstallResult, error) {
	skillsSrc := filepath.Join(installDir, "agents", "skills")
	home, _ := os.UserHomeDir()
	skillsDst := filepath.Join(home, ".agents", "skills")

	if err := os.MkdirAll(skillsDst, 0o755); err != nil {
		return agentInstallResult{}, fmt.Errorf("creating skills directory: %w", err)
	}

	skills := []string{
		"opscompanion-init",
		"opscompanion-context",
		"opscompanion-search",
		"opscompanion-remember",
	}

	for _, skill := range skills {
		link := filepath.Join(skillsDst, skill)
		target := filepath.Join(skillsSrc, skill)
		_ = os.RemoveAll(link)
		if err := os.Symlink(target, link); err != nil {
			return agentInstallResult{}, fmt.Errorf("symlinking %s: %w", skill, err)
		}
	}

	rulesPath, added, err := ensureCodexOPCRule(home)
	if err != nil {
		return agentInstallResult{}, err
	}

	result := agentInstallResult{
		CodexRulesPath:  rulesPath,
		CodexRulesAdded: added,
	}

	if plan != nil && plan.InstallCodexRepoSkills {
		commandText, err := runCodexRepoSkillsInstallQuiet(plan.CodexRepoRoot, plan.CodexRunner)
		result.RepoSkillsCmd = commandText
		if err != nil {
			return result, err
		}
	}

	_ = cfg
	return result, nil
}

func ensureCodexOPCRule(home string) (path string, added bool, err error) {
	rulesDir := filepath.Join(home, ".codex", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return "", false, fmt.Errorf("creating codex rules directory: %w", err)
	}

	path = filepath.Join(rulesDir, "default.rules")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", false, fmt.Errorf("reading codex rules: %w", err)
	}
	if strings.Contains(string(data), codexOPCRule) {
		return path, false, nil
	}

	toWrite := appendCodexRule(data)
	if err := os.WriteFile(path, []byte(toWrite), 0644); err != nil {
		return "", false, fmt.Errorf("writing codex rules: %w", err)
	}
	return path, true, nil
}

func appendCodexRule(existing []byte) string {
	content := string(existing)
	if content == "" {
		return codexOPCRule + "\n"
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + codexOPCRule + "\n"
}

// isGitRepo checks if the directory is a git repository.
func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

// gitCmd runs a git command in the given directory.
func gitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitClone clones a repository.
func gitClone(repo, dest string) error {
	cmd := exec.Command("git", "clone", "--quiet", repo, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runShell runs a command, ignoring errors (best-effort, like the bash script).
func runShell(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// humanSkills returns the skill names formatted for display.
func humanSkills() string {
	return strings.Join([]string{
		"/opscompanion-init",
		"/opscompanion-context",
		"/opscompanion-search",
		"/opscompanion-remember",
	}, ", ")
}
