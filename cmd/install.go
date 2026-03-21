package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opscompanion/opc/internal/agent"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

const (
	skillsRepo = "https://github.com/opscompanion/opscompanion-skills.git"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install OpsCompanion skills and hooks for an agent",
	Long: `Sets up OpsCompanion for the specified agent runtime:

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
		return fmt.Errorf("--agent is required for install (e.g. opc install --agent claude)")
	}

	fmt.Println()
	fmt.Println("  opscompanion installer")
	fmt.Println()

	// ── 1. Ensure skills repo ──────────────────────────────────────────────

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	installDir := filepath.Join(home, ".opscompanion", "skills")

	if isGitRepo(installDir) {
		fmt.Println("  Updating skills...")
		if err := gitCmd(installDir, "pull", "--quiet"); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: git pull failed: %v\n", err)
		}
	} else {
		fmt.Println("  Downloading skills...")
		os.RemoveAll(installDir)
		if err := os.MkdirAll(filepath.Dir(installDir), 0755); err != nil {
			return fmt.Errorf("creating skills directory: %w", err)
		}
		if err := gitClone(skillsRepo, installDir); err != nil {
			return fmt.Errorf("cloning skills repo: %w", err)
		}
	}

	// ── 2. Ensure config exists ────────────────────────────────────────────

	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &models.Config{
			APIURL: "https://api.opscompanion.dev/v1",
			APIKey: "mock-key",
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("writing mock config: %w", err)
		}
		fmt.Println("  config: mock mode (run `opc init` to configure)")
	} else {
		p, _ := config.Path()
		fmt.Printf("  config: %s\n", p)
	}

	// ── 3. Agent-specific installation ─────────────────────────────────────

	switch ag.Name {
	case agent.Claude:
		if err := installClaude(installDir); err != nil {
			return err
		}
	case agent.Codex:
		if err := installCodex(installDir); err != nil {
			return err
		}
	default:
		fmt.Printf("  agent %q: no specific installer yet — generating hooks only\n", ag.Name)
	}

	// ── 4. Generate hooks ──────────────────────────────────────────────────

	binary, err := os.Executable()
	if err != nil {
		binary = "opc"
	}

	fmt.Println()
	fmt.Println("  Generating hooks...")
	switch ag.HookFormat {
	case "claude-hooks":
		if err := generateClaudeHooks(binary, ag, cfg.APIKey); err != nil {
			return fmt.Errorf("generating hooks: %w", err)
		}
	case "codex-hooks":
		if err := generateCodexHooks(binary, ag, cfg.APIKey); err != nil {
			return fmt.Errorf("generating hooks: %w", err)
		}
	default:
		generateGenericHooks(binary, ag)
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
	fmt.Println("    /opscompanion-recall     Search team memories")
	fmt.Println("    /opscompanion-remember   Save a decision")
	fmt.Println("    /opscompanion-history    View session history")
	return nil
}

func installCodex(installDir string) error {
	skillsSrc := filepath.Join(installDir, "agents", "skills")
	home, _ := os.UserHomeDir()
	skillsDst := filepath.Join(home, ".agents", "skills")

	if err := os.MkdirAll(skillsDst, 0755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}

	skills := []string{
		"opscompanion-init",
		"opscompanion-context",
		"opscompanion-recall",
		"opscompanion-remember",
		"opscompanion-history",
	}

	for _, skill := range skills {
		link := filepath.Join(skillsDst, skill)
		target := filepath.Join(skillsSrc, skill)

		// Remove existing link/dir
		os.RemoveAll(link)

		if err := os.Symlink(target, link); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to symlink %s: %v\n", skill, err)
		}
	}

	fmt.Println()
	fmt.Println("  Installed for Codex.")
	fmt.Println()
	fmt.Println("  Skills (in ~/.agents/skills/):")
	fmt.Println("    $opscompanion-init       Set up OpsCompanion")
	fmt.Println("    $opscompanion-context    Load org/team/user context")
	fmt.Println("    $opscompanion-recall     Search team memories")
	fmt.Println("    $opscompanion-remember   Save a decision")
	fmt.Println("    $opscompanion-history    View session history")
	return nil
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
		"/opscompanion-recall",
		"/opscompanion-remember",
		"/opscompanion-history",
	}, ", ")
}
