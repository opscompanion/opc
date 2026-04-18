package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opscompanion/opc/internal/agent"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/opscompanion/opc/internal/tui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive onboarding for OpsCompanion",
	Long: `Launches an interactive setup wizard that configures your API key,
selects one or more agents, installs skills/plugins, and generates hooks in one flow.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

type setupStage int

const (
	setupStageConfigChoice setupStage = iota
	setupStageAPIKey
	setupStageAgents
	setupStageCodexRepoSkills
	setupStageExecuting
	setupStageDone
	setupStageFailed
)

type setupProgressMsg struct {
	text string
}

type setupDoneMsg struct {
	result setupResult
	err    error
}

type apiKeyVerifyDoneMsg struct {
	seq    int
	result apiKeyVerificationResult
	err    error
}

type apiKeySpinnerMsg struct{}
type openSecureAPIKeyMsg struct{}

type setupModel struct {
	stage           setupStage
	existing        *models.Config
	detected        agent.Info
	repoRoot        string
	inRepo          bool
	runner          packageRunner
	plan            setupPlan
	transcript      []tui.TranscriptEntry
	selectPrompt    *tui.SingleSelectModel
	textPrompt      *tui.TextPromptModel
	multiPrompt     *tui.MultiSelectModel
	confirmPrompt   *tui.ConfirmModel
	progressLines   []string
	result          setupResult
	err             string
	cancelled       bool
	done            bool
	progressChan    chan tea.Msg
	apiKeyVerifySeq int
	apiKeyCandidate string
	apiKeyVerifying bool
	spinnerFrame    int
}

func newSetupModel(existing *models.Config, detected agent.Info, repoRoot string, inRepo bool, runner packageRunner) *setupModel {
	model := &setupModel{
		existing: existing,
		detected: detected,
		repoRoot: repoRoot,
		inRepo:   inRepo,
		runner:   runner,
		plan: setupPlan{
			ConfigMode:    configModeOverwrite,
			CodexRepoRoot: repoRoot,
			CodexRunner:   runner,
		},
	}

	if detected.Name != agent.Unknown && detected.Name != agent.Auto {
		model.plan.Agents = []agent.Info{detected}
	}

	if existing != nil {
		model.stage = setupStageConfigChoice
		model.selectPrompt = &tui.SingleSelectModel{
			Label:   "config",
			Message: "How should I handle the existing config?",
			Options: []tui.SelectOption{
				{Title: "Keep existing", Description: "Reuse the saved config", Value: string(configModeKeep)},
				{Title: "Overwrite", Description: "Replace the saved config", Value: string(configModeOverwrite)},
			},
		}
		return model
	}

	model.beginAPIKeyPrompt()
	return model
}

func runSetup(cmd *cobra.Command, args []string) error {
	if !isInteractiveSession() {
		return fmt.Errorf("`opc setup` requires an interactive terminal")
	}

	existing, err := config.Load()
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	repoRoot, inRepo := findGitRoot(cwd)
	runner := packageRunner{}
	if inRepo {
		runner = detectPackageRunner(repoRoot)
	}

	model := newSetupModel(existing, ActiveAgent, repoRoot, inRepo, runner)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return err
	}

	resultModel, ok := finalModel.(*setupModel)
	if !ok {
		return fmt.Errorf("unexpected setup model type %T", finalModel)
	}
	if resultModel.cancelled {
		fmt.Println("Setup cancelled.")
		return nil
	}
	if resultModel.err != "" && resultModel.stage == setupStageFailed {
		return errors.New(resultModel.err)
	}
	return nil
}

func executeSetupPlan(plan setupPlan, existing *models.Config, progress func(string)) (setupResult, error) {
	var result setupResult
	report := func(msg string) {
		if progress != nil {
			progress(msg)
		}
	}

	cfg := existing
	switch plan.ConfigMode {
	case configModeKeep:
		if cfg == nil {
			return result, fmt.Errorf("cannot keep existing config: no configuration found")
		}
		p, _ := config.Path()
		result.ConfigPath = p
		report("Using existing config")
	case configModeOverwrite:
		report("Verifying API key")
		apiKey := strings.TrimSpace(plan.APIKey)
		savedCfg, whoami, path, err := saveVerifiedConfig(apiKey, plan.APIURL)
		if err != nil {
			return result, err
		}
		cfg = savedCfg
		result.ConfigPath = path
		result.ConfigVerified = true
		result.ConfigOwner = whoami.APIKey.OwnerType
		result.ConfigOrg = whoami.Organization.PublicID
		if whoami.User != nil {
			result.ConfigUser = whoami.User.PublicID
		}
		result.ConfigScopes = append([]string(nil), whoami.APIKey.Scopes...)
		report("Saved verified config")
	}

	report("Preparing OpsCompanion skills")
	if len(orderedUniqueAgents(plan.Agents)) == 0 {
		report("No agents selected")
		return result, nil
	}
	installDir, err := ensureSkillsRepoQuiet()
	if err != nil {
		return result, err
	}
	result.SkillsDir = installDir

	for _, ag := range orderedUniqueAgents(plan.Agents) {
		report("Installing " + agentDisplayName(ag))
		installResult, err := installForAgentQuiet(ag, installDir, cfg, &plan)
		if err != nil {
			return result, err
		}
		if ag.Name == agent.Codex {
			result.CodexRulesPath = installResult.CodexRulesPath
			result.CodexRulesAdded = installResult.CodexRulesAdded
			result.CodexRepoSkillsCommand = installResult.RepoSkillsCmd
		}

		report("Generating hooks for " + agentDisplayName(ag))
		hooksPath, configPath, err := generateHooksForAgentQuiet(ag, cfg)
		if err != nil {
			return result, err
		}
		result.AgentResults = append(result.AgentResults, agentSetupResult{
			Agent:      ag,
			HooksPath:  hooksPath,
			ConfigPath: configPath,
		})
	}

	report("Setup complete")
	return result, nil
}

func (m *setupModel) Init() tea.Cmd {
	if m.stage == setupStageAPIKey && m.textPrompt != nil && m.textPrompt.Secret {
		return scheduleOpenSecureAPIKey()
	}
	return nil
}

func (m *setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case setupProgressMsg:
		m.progressLines = append(m.progressLines, msg.text)
		return m, waitForSetupMsg(m.progressChan)
	case setupDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.stage = setupStageFailed
			m.done = true
			return m, nil
		}
		m.result = msg.result
		m.stage = setupStageDone
		m.done = true
		return m, tea.Quit
	case apiKeyVerifyDoneMsg:
		return m.handleAPIKeyVerifyDone(msg)
	case apiKeySpinnerMsg:
		return m.handleAPIKeySpinner()
	case openSecureAPIKeyMsg:
		if m.stage == setupStageAPIKey && m.textPrompt != nil && m.textPrompt.Secret && !m.apiKeyVerifying {
			return m, m.openSecureAPIKeyInput()
		}
		return m, nil
	case tui.SecretResultMsg:
		return m.handleSecureSecretMsg(msg)
	}

	return m, nil
}

func (m *setupModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.stage {
	case setupStageDone, setupStageFailed:
		return m, tea.Quit
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	}

	switch m.stage {
	case setupStageConfigChoice:
		return m.updateSelect(msg)
	case setupStageAPIKey:
		return m.updateText(msg)
	case setupStageAgents:
		return m.updateMultiselect(msg)
	case setupStageCodexRepoSkills:
		return m.updateConfirm(msg)
	case setupStageExecuting:
		return m, nil
	}

	return m, nil
}

func (m *setupModel) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.selectPrompt.MoveUp()
	case "down", "j":
		m.selectPrompt.MoveDown()
	case "enter":
		selected := m.selectPrompt.Selected()
		m.plan.ConfigMode = configMode(selected.Value)
		m.completePrompt(selected.Title)
		if m.plan.ConfigMode == configModeKeep {
			m.beginAgentsPrompt()
			return m, nil
		}
		m.beginAPIKeyPrompt()
		return m, scheduleOpenSecureAPIKey()
	}
	return m, nil
}

func (m *setupModel) updateText(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.stage == setupStageAPIKey && m.textPrompt.Secret {
		if msg.Type != tea.KeyEnter || m.apiKeyVerifying {
			return m, nil
		}
		return m, m.openSecureAPIKeyInput()
	}

	switch msg.Type {
	case tea.KeyEnter:
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		m.textPrompt.Update(msg)
	default:
		m.textPrompt.Update(msg)
	}
	return m, nil
}

func (m *setupModel) openSecureAPIKeyInput() tea.Cmd {
	m.err = ""
	m.textPrompt.StatusIcon = ""
	m.textPrompt.StatusText = ""
	m.textPrompt.Hint = apiKeyPromptHint()
	return tui.PromptSecureSecretInput("API Key: ")
}

func (m *setupModel) beginAPIKeyVerification(value string) (tea.Model, tea.Cmd) {
	m.apiKeyVerifySeq++
	seq := m.apiKeyVerifySeq
	m.apiKeyCandidate = value
	m.apiKeyVerifying = true
	m.textPrompt.StatusIcon = "-"
	m.textPrompt.StatusText = "Checking API key..."
	m.spinnerFrame = 0
	return m, tea.Batch(verifyAPIKeyCmd(m.plan.APIURL, value, seq), scheduleAPIKeySpinner())
}

func (m *setupModel) handleAPIKeyVerifyDone(msg apiKeyVerifyDoneMsg) (tea.Model, tea.Cmd) {
	if m.stage != setupStageAPIKey || msg.seq != m.apiKeyVerifySeq {
		return m, nil
	}

	m.apiKeyVerifying = false
	if msg.err != nil {
		m.err = ""
		m.textPrompt.StatusIcon = "x"
		m.textPrompt.StatusText = msg.err.Error()
		m.textPrompt.Hint = "Press Enter to try again. Ctrl+C or Ctrl+D to cancel."
		return m, nil
	}

	m.err = ""
	m.textPrompt.StatusIcon = "✓"
	m.textPrompt.StatusText = "API key verified"
	m.textPrompt.Hint = apiKeyPromptHint()
	m.plan.APIKey = m.apiKeyCandidate
	m.plan.APIURL = msg.result.APIURL
	m.transcript = append(m.transcript, tui.TranscriptEntry{
		Answer: apiKeyVerifiedSummary(msg.result.APIURL),
		Icon:   "✓",
	})
	if scopes := apiKeyScopesSummary(msg.result.WhoAmI.APIKey.Scopes); scopes != "" {
		m.transcript = append(m.transcript, tui.TranscriptEntry{
			Answer: scopes,
		})
	}
	m.beginAgentsPrompt()
	return m, nil
}

func (m *setupModel) handleAPIKeySpinner() (tea.Model, tea.Cmd) {
	if m.stage != setupStageAPIKey || !m.apiKeyVerifying {
		return m, nil
	}
	frames := []string{"-", "\\", "|", "/"}
	m.spinnerFrame = (m.spinnerFrame + 1) % len(frames)
	m.textPrompt.StatusIcon = frames[m.spinnerFrame]
	m.textPrompt.StatusText = "Checking API key..."
	return m, scheduleAPIKeySpinner()
}

func scheduleAPIKeySpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return apiKeySpinnerMsg{}
	})
}

func scheduleOpenSecureAPIKey() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
		return openSecureAPIKeyMsg{}
	})
}

func validateAPIKeyInput(value string) string {
	if value == "" {
		return ""
	}
	if strings.TrimSpace(value) != value {
		return "Remove leading or trailing spaces from the API key."
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return "API keys cannot contain spaces."
	}
	return ""
}

func verifyAPIKeyCmd(apiURL string, value string, seq int) tea.Cmd {
	return func() tea.Msg {
		result, err := verifyAPIKey(value, apiURL)
		return apiKeyVerifyDoneMsg{
			seq:    seq,
			result: result,
			err:    err,
		}
	}
}

func (m *setupModel) handleSecureSecretMsg(msg tui.SecretResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		if errors.Is(msg.Err, tui.ErrSecretInputCancelled) {
			m.cancelled = true
			return m, tea.Quit
		}
		m.err = ""
		m.textPrompt.StatusIcon = "x"
		m.textPrompt.StatusText = "Secure input failed"
		m.textPrompt.Hint = "Press Enter to try again. Ctrl+C or Ctrl+D to cancel."
		return m, nil
	}

	value := strings.TrimSpace(msg.Value)
	if value == "" {
		m.err = ""
		m.textPrompt.StatusIcon = "x"
		m.textPrompt.StatusText = "API key is required."
		m.textPrompt.Hint = "Press Enter to try again. Ctrl+C or Ctrl+D to cancel."
		return m, nil
	}

	if localErr := validateAPIKeyInput(value); localErr != "" {
		m.err = ""
		m.textPrompt.StatusIcon = "x"
		m.textPrompt.StatusText = localErr
		m.textPrompt.Hint = "Press Enter to try again. Ctrl+C or Ctrl+D to cancel."
		return m, nil
	}

	return m.beginAPIKeyVerification(value)
}

func (m *setupModel) updateMultiselect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.multiPrompt.MoveUp()
	case "down", "j":
		m.multiPrompt.MoveDown()
	case " ":
		m.multiPrompt.ToggleCurrent()
	case "a":
		m.multiPrompt.ToggleAll()
	case "enter":
		selected := m.collectSelectedAgents()
		if len(selected) == 0 {
			m.err = "Select at least one agent."
			return m, nil
		}
		m.err = ""
		m.plan.Agents = selected
		m.completePrompt(agentNamesSummary(selected))
		if m.inRepo && planIncludesAgent(m.plan, agent.Codex) {
			m.beginCodexConfirm()
			return m, nil
		}
		return m.beginExecution()
	}
	return m, nil
}

func (m *setupModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h", "y":
		m.confirmPrompt.Selected = 0
	case "right", "l", "n":
		m.confirmPrompt.Selected = 1
	case "up", "k":
		m.confirmPrompt.Selected = 0
	case "down", "j":
		m.confirmPrompt.Selected = 1
	case "enter", " ":
		m.plan.InstallCodexRepoSkills = m.confirmPrompt.YesSelected()
		answer := "No"
		if m.plan.InstallCodexRepoSkills {
			answer = "Yes"
		}
		m.completePrompt(answer)
		return m.beginExecution()
	}
	return m, nil
}

func (m *setupModel) beginAPIKeyPrompt() {
	m.stage = setupStageAPIKey
	m.textPrompt = &tui.TextPromptModel{
		Label:       "api key",
		Message:     "What is your OpsCompanion API key?",
		Description: "Dashboard > Profile > Manage Account > API Keys > Personal API Key Tokens\nMake sure the API key has the permissions you need for your use case.",
		InputLabel:  "Press Enter to submit key securely",
		Secret:      true,
	}
	m.textPrompt.Hint = apiKeyPromptHint()
	m.err = ""
}

func apiKeyPromptHint() string {
	return "Paste is supported in secure input."
}

func apiKeyVerifiedSummary(apiURL string) string {
	if config.IsDevAPIURL(apiURL) {
		return "API key verified & saved (DEV KEY)"
	}
	return "API key verified & saved"
}

func apiKeyScopesSummary(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	const wrapAt = 72
	const continuationIndent = "        "

	label := "scopes"
	if len(scopes) == 1 {
		label = "scope"
	}
	line := fmt.Sprintf("%d %s:", len(scopes), label)
	lines := []string{line}
	currentLen := len(line)

	for i, scope := range scopes {
		segment := scope
		if i == 0 {
			segment = " " + scope
		} else {
			segment = ", " + scope
		}

		if currentLen+len(segment) > wrapAt && currentLen > len(line) {
			lines = append(lines, continuationIndent+scope)
			currentLen = len(continuationIndent) + len(scope)
			continue
		}

		lines[len(lines)-1] += segment
		currentLen += len(segment)
	}

	return strings.Join(lines, "\n")
}

func (m *setupModel) beginAgentsPrompt() {
	m.stage = setupStageAgents
	options := make([]tui.SelectOption, 0, len(setupAgentOrder()))
	for _, ag := range setupAgentOrder() {
		description := "Skills only"
		switch ag.Name {
		case agent.Claude:
			description = "Install plugin registration, hooks, and OTEL"
		case agent.Codex:
			description = "Install skills, rules, hooks, and OTEL"
		}
		options = append(options, tui.SelectOption{
			Title:       agentDisplayName(ag),
			Description: description,
			Value:       string(ag.Name),
		})
	}
	selected := make(map[string]bool, len(m.plan.Agents))
	for _, ag := range m.plan.Agents {
		selected[string(ag.Name)] = true
	}
	m.multiPrompt = &tui.MultiSelectModel{
		Label:         "agents",
		Message:       "Select the agents to configure:",
		Hint:          "Space: toggle • A: all • Enter: confirm",
		Options:       options,
		SelectedValue: selected,
	}
	m.err = ""
}

func (m *setupModel) beginCodexConfirm() {
	m.stage = setupStageCodexRepoSkills
	m.confirmPrompt = tui.NewConfirmModel("confirm",
		// Keep the generic confirm label here since the component is reused elsewhere.
		fmt.Sprintf("Install Codex repo-local skills in %s using `%s skills add opscompanion/opscompanion-skills`?", m.repoRoot, m.runner.Display),
		false,
	)
	m.err = ""
}

func (m *setupModel) beginExecution() (tea.Model, tea.Cmd) {
	m.stage = setupStageExecuting
	m.progressLines = nil
	progressCh := make(chan tea.Msg)
	m.progressChan = progressCh
	go func(plan setupPlan, existing *models.Config) {
		result, err := executeSetupPlan(plan, existing, func(text string) {
			progressCh <- setupProgressMsg{text: text}
		})
		progressCh <- setupDoneMsg{result: result, err: err}
		close(progressCh)
	}(m.plan, m.existing)
	return m, waitForSetupMsg(progressCh)
}

func (m *setupModel) collectSelectedAgents() []agent.Info {
	selected := make([]agent.Info, 0, len(m.multiPrompt.Options))
	for _, option := range m.multiPrompt.Selected() {
		if option.Value != "" {
			selected = append(selected, agent.Resolve(option.Value))
		}
	}
	return orderedUniqueAgents(selected)
}

func (m *setupModel) completePrompt(answer string) {
	label := ""
	message := ""
	switch m.stage {
	case setupStageConfigChoice:
		label = m.selectPrompt.Label
		message = m.selectPrompt.Message
	case setupStageAPIKey:
		label = m.textPrompt.Label
		message = m.textPrompt.Message
	case setupStageAgents:
		label = m.multiPrompt.Label
		message = m.multiPrompt.Message
	case setupStageCodexRepoSkills:
		label = m.confirmPrompt.Label
		message = m.confirmPrompt.Message
	}
	m.transcript = append(m.transcript, tui.TranscriptEntry{
		Label:   label,
		Message: message,
		Answer:  answer,
	})
	m.err = ""
}

func (m *setupModel) View() string {
	var sections []string
	sections = append(sections, tui.RenderIntro("setup", "OpsCompanion", "guided configuration for agents, hooks, and repo skills"))
	for _, entry := range m.transcript {
		sections = append(sections, tui.RenderCompletedPrompt(entry))
	}

	switch m.stage {
	case setupStageConfigChoice:
		m.selectPrompt.Err = m.err
		sections = append(sections, m.selectPrompt.View())
	case setupStageAPIKey:
		m.textPrompt.Err = m.err
		sections = append(sections, m.textPrompt.View())
	case setupStageAgents:
		m.multiPrompt.Err = m.err
		sections = append(sections, m.multiPrompt.View())
	case setupStageCodexRepoSkills:
		m.confirmPrompt.Err = m.err
		sections = append(sections, m.confirmPrompt.View())
	case setupStageExecuting:
		sections = append(sections, tui.RenderSpinnerPrompt("spinner", "Configuring OpsCompanion", m.progressLines))
	case setupStageDone:
		sections = append(sections, renderSuccessOutro(m.plan, m.result))
	case setupStageFailed:
		sections = append(sections, renderFailureOutro(m.err))
	}

	return strings.Join(sections, "\n\n")
}

func waitForSetupMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func renderSuccessOutro(plan setupPlan, result setupResult) string {
	lines := []string{
		tui.RenderTag("complete"),
		"",
		tui.RenderPrimary("OpsCompanion is ready."),
		tui.RenderMuted("Configured: " + agentNamesSummary(plan.Agents)),
	}
	if result.ConfigPath != "" {
		lines = append(lines, tui.RenderMuted("Config: "+result.ConfigPath))
	}
	for _, ag := range result.AgentResults {
		summary := agentDisplayName(ag.Agent)
		if ag.HooksPath != "" {
			summary += " hooks: " + ag.HooksPath
		}
		if ag.ConfigPath != "" {
			summary += " • config: " + ag.ConfigPath
		}
		if ag.HooksPath == "" && ag.ConfigPath == "" {
			summary += ": skills only"
		}
		lines = append(lines, tui.RenderMuted(summary))
	}
	if result.CodexRulesPath != "" {
		ruleLine := "Codex rule present: " + result.CodexRulesPath
		if result.CodexRulesAdded {
			ruleLine = "Codex rule added: " + result.CodexRulesPath
		}
		lines = append(lines, tui.RenderMuted(ruleLine))
	}
	if result.CodexRepoSkillsCommand != "" {
		lines = append(lines, tui.RenderMuted("Repo skills: "+result.CodexRepoSkillsCommand))
	}
	lines = append(lines, "")
	lines = append(lines, tui.RenderMuted("To add OpsCompanion skills to another project later, run `npx skills add opscompanion/opscompanion-skills` from that repo root."))
	lines = append(lines, tui.RenderMuted("If the repo uses Bun or pnpm, use `bunx`, `pnpx`, or `pnpm dlx` instead."))
	lines = append(lines, tui.RenderMuted("Restart your agent for changes to take effect."))
	return strings.Join(lines, "\n")
}

func renderFailureOutro(err string) string {
	return tui.RenderTag("error") + "\n\n" +
		tui.RenderPrimary("Setup failed.") + "\n" +
		tui.RenderError(err)
}

func maskSetupValue(value string, secret bool, kept bool) string {
	if kept {
		return "(kept existing value)"
	}
	if secret {
		if value == "" {
			return ""
		}
		return strings.Repeat("*", len(value))
	}
	return value
}

func runAgentSelectModel() (agent.Info, error) {
	model := &agentSelectModel{
		options: setupAgentOrder(),
		selectPrompt: &tui.SingleSelectModel{
			Label:   "select",
			Message: "Select the agent to configure:",
		},
	}

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return agent.Info{}, err
	}
	result, ok := finalModel.(*agentSelectModel)
	if !ok {
		return agent.Info{}, fmt.Errorf("unexpected agent selection model type %T", finalModel)
	}
	if result.cancelled {
		return agent.Info{}, fmt.Errorf("install cancelled")
	}
	return result.options[result.selectPrompt.Index], nil
}

type agentSelectModel struct {
	options      []agent.Info
	selectPrompt *tui.SingleSelectModel
	cancelled    bool
}

func (m *agentSelectModel) Init() tea.Cmd { return nil }

func (m *agentSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		m.selectPrompt.MoveUp()
	case "down", "j":
		m.selectPrompt.MoveDown()
	case "enter":
		return m, tea.Quit
	}
	return m, nil
}

func (m *agentSelectModel) View() string {
	items := make([]tui.SelectOption, 0, len(m.options))
	for _, ag := range m.options {
		description := "Generate generic hooks only"
		switch ag.Name {
		case agent.Claude:
			description = "Install plugin registration and hooks"
		case agent.Codex:
			description = "Install skills, rules, and hooks"
		}
		items = append(items, tui.SelectOption{
			Title:       agentDisplayName(ag),
			Description: description,
			Value:       string(ag.Name),
		})
	}
	m.selectPrompt.Options = items
	return m.selectPrompt.View()
}
