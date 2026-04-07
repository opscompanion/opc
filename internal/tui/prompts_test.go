package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSingleSelectModelNavigation(t *testing.T) {
	model := &SingleSelectModel{
		Options: []SelectOption{
			{Title: "One", Value: "one"},
			{Title: "Two", Value: "two"},
		},
	}

	model.MoveDown()
	if got := model.Selected().Value; got != "two" {
		t.Fatalf("selected = %q", got)
	}
	model.MoveUp()
	if got := model.Selected().Value; got != "one" {
		t.Fatalf("selected = %q", got)
	}
}

func TestMultiSelectModelToggleAndToggleAll(t *testing.T) {
	model := &MultiSelectModel{
		Options: []SelectOption{
			{Title: "One", Value: "one"},
			{Title: "Two", Value: "two"},
		},
		SelectedValue: map[string]bool{},
	}

	model.ToggleCurrent()
	if !model.SelectedValue["one"] {
		t.Fatal("current option should be selected")
	}
	model.ToggleAll()
	if !model.SelectedValue["one"] || !model.SelectedValue["two"] {
		t.Fatalf("toggle all should select all: %#v", model.SelectedValue)
	}
	model.ToggleAll()
	if model.SelectedValue["one"] || model.SelectedValue["two"] {
		t.Fatalf("second toggle all should clear all: %#v", model.SelectedValue)
	}
}

func TestConfirmModelUpdate(t *testing.T) {
	model := NewConfirmModel("confirm", "Proceed?", false)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got := updated.(*ConfirmModel)
	if !got.YesSelected() {
		t.Fatal("y should select yes")
	}
}

func TestRenderPromptFrameUsesSharedVisuals(t *testing.T) {
	view := RenderPromptFrame("select", "Pick one")
	if !strings.Contains(view, "◆") || !strings.Contains(view, "Pick one") {
		t.Fatalf("RenderPromptFrame() = %q", view)
	}
}

func TestTextPromptModelSecretView(t *testing.T) {
	view := (&TextPromptModel{
		Label:       "api key",
		Message:     "What is your secret?",
		Description: "Dashboard > Profile > Manage Account > API Keys > Personal API Key Tokens\nMake sure the API key has the permissions you need for your use case.",
		InputLabel:  "Press Enter to submit key securely",
		Secret:      true,
		Hint:        "Paste is supported in secure input.",
		StatusIcon:  "✓",
		StatusText:  "API key verified",
	}).View()
	if !strings.Contains(view, "api key") || !strings.Contains(view, "> Press Enter to submit key securely") || !strings.Contains(view, "Dashboard > Profile > Manage Account > API Keys > Personal API Key Tokens") || !strings.Contains(view, "Make sure the API key has the permissions you need for your use case.") || !strings.Contains(view, "Paste is supported in secure input.") || !strings.Contains(view, "API key verified") {
		t.Fatalf("secret view = %q", view)
	}
	if strings.Index(view, "Dashboard > Profile > Manage Account > API Keys > Personal API Key Tokens") > strings.Index(view, "> Press Enter to submit key securely") {
		t.Fatalf("description should render before input row: %q", view)
	}
	if strings.Contains(view, "█") {
		t.Fatalf("secret view should not render inline cursor: %q", view)
	}
}

func TestTextPromptModelUpdateUsesRawRunesForPaste(t *testing.T) {
	model := &TextPromptModel{}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("opc_pasted_key"), Paste: true})
	if model.Value != "opc_pasted_key" {
		t.Fatalf("pasted value = %q", model.Value)
	}
}

func TestRenderCompletedPromptStatusOnly(t *testing.T) {
	view := RenderCompletedPrompt(TranscriptEntry{
		Answer: "API key verified & saved",
		Icon:   "✓",
	})
	if !strings.Contains(view, "API key verified & saved") {
		t.Fatalf("status view = %q", view)
	}
	if strings.Contains(view, "\n") {
		t.Fatalf("status-only transcript should be a single line: %q", view)
	}
}
