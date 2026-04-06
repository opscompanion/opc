package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
)

func TestRunResetNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	output, err := captureResetRun(t, "")
	if err != nil {
		t.Fatalf("runReset: %v", err)
	}
	if !strings.Contains(output, "No saved config found") {
		t.Fatalf("output = %q", output)
	}
}

func TestRunResetAbortsWithoutConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := config.Save(&models.Config{APIKey: "test-key", APIURL: config.DefaultAPIURL}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	output, err := captureResetRun(t, "n\n")
	if err != nil {
		t.Fatalf("runReset: %v", err)
	}
	if !strings.Contains(output, "This cannot be undone.") {
		t.Fatalf("output = %q", output)
	}
	if !strings.Contains(output, "Aborted.") {
		t.Fatalf("output = %q", output)
	}
	if cfg, err := config.Load(); err != nil || cfg == nil {
		t.Fatalf("config should still exist, cfg=%v err=%v", cfg, err)
	}
}

func TestRunResetDeletesConfigAfterConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := config.Save(&models.Config{APIKey: "test-key", APIURL: config.DefaultAPIURL}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	output, err := captureResetRun(t, "y\n")
	if err != nil {
		t.Fatalf("runReset: %v", err)
	}
	if !strings.Contains(output, "Deleted config:") {
		t.Fatalf("output = %q", output)
	}
	if cfg, err := config.Load(); err != nil || cfg != nil {
		t.Fatalf("config should be deleted, cfg=%v err=%v", cfg, err)
	}
}

func captureResetRun(t *testing.T, input string) (string, error) {
	t.Helper()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inReader, inWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdin: %v", err)
	}
	if _, err := io.WriteString(inWriter, input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = inWriter.Close()
	os.Stdin = inReader

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdout: %v", err)
	}
	os.Stdout = outWriter

	runErr := runReset(resetCmd, nil)

	_ = outWriter.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, outReader); err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	return buf.String(), runErr
}
