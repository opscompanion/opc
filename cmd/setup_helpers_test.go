package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
)

func TestFindGitRoot(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	nested := filepath.Join(repo, "a", "b")

	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll .git: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll nested: %v", err)
	}

	got, ok := findGitRoot(nested)
	if !ok {
		t.Fatal("findGitRoot() ok = false, want true")
	}
	if got != repo {
		t.Fatalf("findGitRoot() = %q, want %q", got, repo)
	}
}

func TestDetectPackageRunnerPrefersBunxForBunRepos(t *testing.T) {
	repo := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", bin)

	mustWriteFile(t, filepath.Join(repo, "bun.lockb"), "")
	mustWriteExecutable(t, filepath.Join(bin, "bunx"))
	mustWriteExecutable(t, filepath.Join(bin, "npx"))

	got := detectPackageRunner(repo)
	if got.Command != "bunx" || got.Display != "bunx" {
		t.Fatalf("detectPackageRunner() = %+v, want bunx", got)
	}
}

func TestDetectPackageRunnerFallsBackToPnpmDlx(t *testing.T) {
	repo := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", bin)

	mustWriteFile(t, filepath.Join(repo, "pnpm-lock.yaml"), "")
	mustWriteExecutable(t, filepath.Join(bin, "pnpm"))

	got := detectPackageRunner(repo)
	if got.Command != "pnpm" || len(got.Args) != 1 || got.Args[0] != "dlx" || got.Display != "pnpm dlx" {
		t.Fatalf("detectPackageRunner() = %+v, want pnpm dlx", got)
	}
}

func TestDetectPackageRunnerDefaultsToNpx(t *testing.T) {
	repo := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", bin)

	mustWriteExecutable(t, filepath.Join(bin, "npx"))

	got := detectPackageRunner(repo)
	if got.Command != "npx" || got.Display != "npx" {
		t.Fatalf("detectPackageRunner() = %+v, want npx", got)
	}
}

func TestResolveSetupAPIURL(t *testing.T) {
	existing := &models.Config{APIURL: "https://saved.example/v1/"}
	if got := resolveSetupAPIURL("", existing); got != "https://saved.example/v1" {
		t.Fatalf("resolveSetupAPIURL(existing) = %q", got)
	}

	t.Setenv("OPSCOMPANION_API_URL", "https://env.example/v1/")
	if got := resolveSetupAPIURL("", existing); got != "https://env.example/v1" {
		t.Fatalf("resolveSetupAPIURL(env) = %q", got)
	}

	t.Setenv("OPSCOMPANION_API_URL", "")
	if got := resolveSetupAPIURL("", nil); got != config.DefaultAPIURL {
		t.Fatalf("resolveSetupAPIURL(default) = %q", got)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func mustWriteExecutable(t *testing.T, path string) {
	t.Helper()
	mustWriteFile(t, path, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("Chmod(%q): %v", path, err)
	}
}
