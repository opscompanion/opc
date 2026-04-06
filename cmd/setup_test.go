package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opscompanion/opc/internal/agent"
	"github.com/opscompanion/opc/internal/models"
	"github.com/opscompanion/opc/internal/tui"
)

func TestSetupMultiselectSpaceAndAllToggle(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAgentsPrompt()

	if got := len(model.collectSelectedAgents()); got != 0 {
		t.Fatalf("initial selected agents = %d, want 0", got)
	}

	if _, _ = model.updateMultiselect(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}); !model.multiPrompt.SelectedValue[string(agent.Claude)] {
		t.Fatal("space should toggle the highlighted agent on")
	}

	if _, _ = model.updateMultiselect(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}); len(model.collectSelectedAgents()) != len(setupAgentOrder()) {
		t.Fatalf("a should select all agents, got %d", len(model.collectSelectedAgents()))
	}

	if _, _ = model.updateMultiselect(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}); len(model.collectSelectedAgents()) != 0 {
		t.Fatalf("second a should clear all agents, got %d", len(model.collectSelectedAgents()))
	}
}

func TestSetupMultiselectRequiresSelection(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAgentsPrompt()

	updated, _ := model.updateMultiselect(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*setupModel)
	if got.err == "" || !strings.Contains(got.err, "Select at least one agent") {
		t.Fatalf("empty multiselect error = %q", got.err)
	}
}

func TestSetupMultiselectSummaryUsesStableOrder(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAgentsPrompt()
	model.multiPrompt.SelectedValue[string(agent.OpenClaw)] = true
	model.multiPrompt.SelectedValue[string(agent.Claude)] = true
	model.multiPrompt.SelectedValue[string(agent.Codex)] = true

	summary := agentNamesSummary(model.collectSelectedAgents())
	if summary != "Claude, Codex, OpenClaw" {
		t.Fatalf("agentNamesSummary() = %q", summary)
	}
}

func TestSetupFlowShowsCodexConfirmForRepoSelection(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "/tmp/repo", true, packageRunner{Display: "npx"})
	model.beginAgentsPrompt()
	model.multiPrompt.SelectedValue[string(agent.Codex)] = true

	updated, _ := model.updateMultiselect(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*setupModel)
	if got.stage != setupStageCodexRepoSkills {
		t.Fatalf("stage = %v, want codex confirm", got.stage)
	}
}

func TestSetupSecureSecretMsgAdvancesToAPIURL(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAPIKeyPrompt()
	if model.textPrompt.Label != "api key" {
		t.Fatalf("textPrompt.Label = %q", model.textPrompt.Label)
	}

	updated, cmd := model.handleSecureSecretMsg(tui.SecretResultMsg{Value: "mock-key"})
	got := updated.(*setupModel)
	if got.stage != setupStageAPIKey {
		t.Fatalf("stage = %v, want api key while verifying", got.stage)
	}
	if cmd == nil {
		t.Fatal("expected verify command after secure input")
	}
	if !got.apiKeyVerifying {
		t.Fatal("expected api key verification to be active")
	}
}

func TestSetupInitAutoOpensSecureInput(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	if cmd := model.Init(); cmd == nil {
		t.Fatal("expected secure input command on initial api key step")
	}
}

func TestSetupConfigChoiceAutoOpensSecureInput(t *testing.T) {
	model := newSetupModel(&models.Config{APIKey: "existing", APIURL: "https://api.opscompanion.ai/v1"}, "", agent.Info{}, "", false, packageRunner{})
	model.selectPrompt.Index = 1
	updated, cmd := model.updateSelect(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*setupModel)
	if got.stage != setupStageAPIKey {
		t.Fatalf("stage = %v, want api key", got.stage)
	}
	if cmd == nil {
		t.Fatal("expected secure input command when moving to api key step")
	}
}

func TestSetupAPIKeyVerifyDoneAdvancesToAPIURL(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAPIKeyPrompt()
	model.apiKeyCandidate = "mock-key"
	model.apiKeyVerifySeq = 1
	model.apiKeyVerifying = true

	updated, _ := model.handleAPIKeyVerifyDone(apiKeyVerifyDoneMsg{seq: 1})
	got := updated.(*setupModel)
	if got.stage != setupStageAPIURL {
		t.Fatalf("stage = %v, want api url", got.stage)
	}
	if got.plan.APIKey != "mock-key" {
		t.Fatalf("plan.APIKey = %q", got.plan.APIKey)
	}
	if len(got.transcript) == 0 || !strings.Contains(got.transcript[0].Answer, "*") {
		t.Fatalf("transcript = %#v", got.transcript)
	}
}

func TestSetupSecureSecretCancelQuits(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAPIKeyPrompt()
	updated, cmd := model.handleSecureSecretMsg(tui.SecretResultMsg{Err: tui.ErrSecretInputCancelled})
	got := updated.(*setupModel)
	if !got.cancelled {
		t.Fatal("expected setup to be marked cancelled")
	}
	if cmd == nil {
		t.Fatal("expected quit command on secure input cancel")
	}
}

func TestSetupSecureSecretEmptyValueShowsRetryHint(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	model.beginAPIKeyPrompt()
	updated, _ := model.handleSecureSecretMsg(tui.SecretResultMsg{Value: ""})
	got := updated.(*setupModel)
	if got.textPrompt.StatusText != "API key is required." {
		t.Fatalf("status text = %q", got.textPrompt.StatusText)
	}
	if got.textPrompt.Hint != "Press Enter to try again. Ctrl+C or Ctrl+D to cancel." {
		t.Fatalf("hint = %q", got.textPrompt.Hint)
	}
	if got.err != "" {
		t.Fatalf("err should be empty to avoid duplicate rendering, got %q", got.err)
	}
}

func TestRenderCompletedPromptShowsCompactAnswer(t *testing.T) {
	view := tui.RenderCompletedPrompt(tui.TranscriptEntry{
		Label:   "text",
		Message: "What is your OpsCompanion API key?",
		Answer:  "********",
	})
	if !strings.Contains(view, "What is your OpsCompanion API key?") || !strings.Contains(view, "********") {
		t.Fatalf("renderCompletedPrompt() = %q", view)
	}
}

func TestRenderMultiselectPromptHighlightsSelected(t *testing.T) {
	view := (&tui.MultiSelectModel{
		Label:   "multiselect",
		Message: "Select the agents to configure:",
		Hint:    "Space: toggle",
		Options: []tui.SelectOption{
			{Title: "Claude", Description: "Install plugin registration and hooks", Value: "claude"},
			{Title: "Codex", Description: "Install skills, rules, and hooks", Value: "codex"},
		},
		Index:         1,
		SelectedValue: map[string]bool{"codex": true},
	}).View()

	if !strings.Contains(view, "◉") || !strings.Contains(view, "Codex") {
		t.Fatalf("renderMultiselectPrompt() = %q", view)
	}
}

func TestExecuteSetupPlanKeepExistingRequiresConfig(t *testing.T) {
	_, err := executeSetupPlan(setupPlan{
		ConfigMode: configModeKeep,
		Agents:     []agent.Info{agent.Resolve(string(agent.Claude))},
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot keep existing config") {
		t.Fatalf("executeSetupPlan() error = %v", err)
	}
}

func TestExecuteSetupPlanKeepExistingReportsProgress(t *testing.T) {
	var progress []string
	result, err := executeSetupPlan(setupPlan{
		ConfigMode: configModeKeep,
		Agents:     []agent.Info{},
	}, &models.Config{APIKey: "mock-key", APIURL: "https://api.opscompanion.ai/v1"}, func(msg string) {
		progress = append(progress, msg)
	})
	if err != nil {
		t.Fatalf("executeSetupPlan() error = %v", err)
	}
	if result.ConfigPath == "" {
		t.Fatal("expected config path for keep-existing plan")
	}
	if len(progress) == 0 || progress[0] != "Using existing config" {
		t.Fatalf("progress = %#v", progress)
	}
}

func TestSetupDoneMessageQuitsImmediately(t *testing.T) {
	model := newSetupModel(nil, "", agent.Info{}, "", false, packageRunner{})
	updated, cmd := model.Update(setupDoneMsg{
		result: setupResult{
			AgentResults: []agentSetupResult{
				{Agent: agent.Resolve(string(agent.Cursor))},
			},
		},
	})
	got := updated.(*setupModel)
	if got.stage != setupStageDone {
		t.Fatalf("stage = %v, want done", got.stage)
	}
	if cmd == nil {
		t.Fatal("expected quit command on setup completion")
	}
}

func TestRenderSuccessOutroHandlesGenericAgents(t *testing.T) {
	view := renderSuccessOutro(setupPlan{
		Agents: []agent.Info{agent.Resolve(string(agent.Cursor))},
	}, setupResult{
		AgentResults: []agentSetupResult{
			{Agent: agent.Resolve(string(agent.Cursor))},
		},
	})
	if !strings.Contains(view, "Cursor: generic hook commands available") {
		t.Fatalf("renderSuccessOutro() = %q", view)
	}
	if !strings.Contains(view, "npx skills add opscompanion/opscompanion-skills") {
		t.Fatalf("renderSuccessOutro() missing future-project note: %q", view)
	}
}
