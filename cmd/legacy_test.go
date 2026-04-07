package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/opscompanion/opc/internal/agent"
)

func TestRunInitPrintsLegacyWarningFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	output, _ := captureStdoutAndRun(t, func() error {
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		inReader, inWriter, err := os.Pipe()
		if err != nil {
			t.Fatalf("stdin pipe: %v", err)
		}
		_ = inWriter.Close()
		os.Stdin = inReader
		return runInit(initCmd, nil)
	})

	firstLine := strings.Split(strings.TrimRight(output, "\n"), "\n")[0]
	if firstLine != legacyCommandWarning {
		t.Fatalf("first line = %q", firstLine)
	}
}

func TestRunInstallPrintsLegacyWarningFirst(t *testing.T) {
	oldAgent := ActiveAgent
	ActiveAgent = agent.Info{}
	defer func() { ActiveAgent = oldAgent }()

	home := t.TempDir()
	t.Setenv("HOME", home)

	output, _ := captureStdoutAndRun(t, func() error {
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		inReader, inWriter, err := os.Pipe()
		if err != nil {
			t.Fatalf("stdin pipe: %v", err)
		}
		_ = inWriter.Close()
		os.Stdin = inReader
		return runInstall(installCmd, nil)
	})

	firstLine := strings.Split(strings.TrimRight(output, "\n"), "\n")[0]
	if firstLine != legacyCommandWarning {
		t.Fatalf("first line = %q", firstLine)
	}
}

func captureStdoutAndRun(t *testing.T, run func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = outWriter

	runErr := run()

	_ = outWriter.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, outReader); err != nil {
		t.Fatalf("stdout read: %v", err)
	}
	return buf.String(), runErr
}
