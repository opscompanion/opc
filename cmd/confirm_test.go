package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opscompanion/opc/internal/tui"
)

func TestPromptConfirmFallbackDefaults(t *testing.T) {
	got, err := captureFallbackConfirm(t, "\n", false)
	if err != nil {
		t.Fatalf("promptConfirmFallback: %v", err)
	}
	if got {
		t.Fatal("blank fallback answer should respect default no")
	}

	got, err = captureFallbackConfirm(t, "\n", true)
	if err != nil {
		t.Fatalf("promptConfirmFallback: %v", err)
	}
	if !got {
		t.Fatal("blank fallback answer should respect default yes")
	}
}

func TestConfirmModelNavigation(t *testing.T) {
	model := tui.NewConfirmModel("confirm", "Delete the saved config?", false)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got := updated.(*tui.ConfirmModel)
	if got.Selected != 0 {
		t.Fatalf("selected = %d, want yes", got.Selected)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got = updated.(*tui.ConfirmModel)
	if got.Selected != 1 {
		t.Fatalf("selected = %d, want no", got.Selected)
	}
}

func TestConfirmModelViewUsesClackStyleYesNo(t *testing.T) {
	view := tui.NewConfirmModel("confirm", "Overwrite the saved config?", true).View()

	if !strings.Contains(view, "● Yes") || !strings.Contains(view, "○ No") {
		t.Fatalf("confirm view = %q", view)
	}
}

func captureFallbackConfirm(t *testing.T, input string, defaultYes bool) (bool, error) {
	t.Helper()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inReader, inWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if _, err := io.WriteString(inWriter, input); err != nil {
		t.Fatalf("stdin write: %v", err)
	}
	_ = inWriter.Close()
	os.Stdin = inReader

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = outWriter

	result, runErr := promptConfirmFallback("Proceed?", defaultYes)

	_ = outWriter.Close()
	_, _ = io.ReadAll(outReader)
	return result, runErr
}
