package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SelectOption struct {
	Title       string
	Description string
	Value       string
}

type TranscriptEntry struct {
	Label   string
	Message string
	Answer  string
}

type TextPromptModel struct {
	Label       string
	Message     string
	Description string
	InputLabel  string
	Value       string
	Secret      bool
	Hint        string
	StatusIcon  string
	StatusText  string
	Err         string
}

func (m *TextPromptModel) Update(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.Value) > 0 {
			m.Value = m.Value[:len(m.Value)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.Value += string(msg.Runes)
		}
	}
}

func (m TextPromptModel) View() string {
	frameLines := []string{RenderPrimary(m.Message)}
	if m.Description != "" {
		for _, line := range strings.Split(m.Description, "\n") {
			frameLines = append(frameLines, RenderMuted(line))
		}
	}

	if m.Secret {
		lines := []string{RenderPromptFrameBlock(m.Label, frameLines)}
		lines = append(lines, inputStyle.Render("> ")+RenderMuted(m.InputLabel))
		if m.StatusText != "" {
			status := m.StatusText
			if m.StatusIcon != "" {
				status = m.StatusIcon + " " + status
			}
			lines = append(lines, renderStatus(status, m.StatusIcon))
		}
		if m.Hint != "" {
			lines = append(lines, RenderMuted(m.Hint))
		}
		if m.Err != "" {
			lines = append(lines, RenderError(m.Err))
		}
		return strings.Join(lines, "\n")
	}

	display := m.Value
	if display == "" {
		display = RenderMuted(m.InputLabel)
	}
	cursor := cursorStyle.Render("█")

	lines := []string{
		RenderPromptFrameBlock(m.Label, frameLines),
	}
	lines = append(lines, inputStyle.Render("> ")+display+cursor)
	if m.Hint != "" {
		lines = append(lines, RenderMuted(m.Hint))
	}
	if m.Err != "" {
		lines = append(lines, RenderError(m.Err))
	}
	return strings.Join(lines, "\n")
}

type SingleSelectModel struct {
	Label   string
	Message string
	Hint    string
	Err     string
	Options []SelectOption
	Index   int
}

func (m *SingleSelectModel) MoveUp() {
	if m.Index > 0 {
		m.Index--
	}
}

func (m *SingleSelectModel) MoveDown() {
	if m.Index < len(m.Options)-1 {
		m.Index++
	}
}

func (m *SingleSelectModel) Selected() SelectOption {
	if len(m.Options) == 0 {
		return SelectOption{}
	}
	return m.Options[m.Index]
}

func (m SingleSelectModel) View() string {
	lines := []string{RenderPromptFrame(m.Label, m.Message)}
	for i, option := range m.Options {
		lines = append(lines, renderSelectOption(option, i == m.Index))
	}
	if m.Hint != "" {
		lines = append(lines, RenderMuted(m.Hint))
	}
	if m.Err != "" {
		lines = append(lines, RenderError(m.Err))
	}
	return strings.Join(lines, "\n")
}

type MultiSelectModel struct {
	Label         string
	Message       string
	Hint          string
	Err           string
	Options       []SelectOption
	Index         int
	SelectedValue map[string]bool
}

func (m *MultiSelectModel) MoveUp() {
	if m.Index > 0 {
		m.Index--
	}
}

func (m *MultiSelectModel) MoveDown() {
	if m.Index < len(m.Options)-1 {
		m.Index++
	}
}

func (m *MultiSelectModel) ToggleCurrent() {
	if len(m.Options) == 0 {
		return
	}
	value := m.Options[m.Index].Value
	m.SelectedValue[value] = !m.SelectedValue[value]
}

func (m *MultiSelectModel) ToggleAll() {
	allSelected := true
	for _, option := range m.Options {
		if !m.SelectedValue[option.Value] {
			allSelected = false
			break
		}
	}
	for _, option := range m.Options {
		m.SelectedValue[option.Value] = !allSelected
	}
}

func (m *MultiSelectModel) Selected() []SelectOption {
	selected := make([]SelectOption, 0, len(m.Options))
	for _, option := range m.Options {
		if m.SelectedValue[option.Value] {
			selected = append(selected, option)
		}
	}
	return selected
}

func (m MultiSelectModel) View() string {
	lines := []string{RenderPromptFrame(m.Label, m.Message)}
	for i, option := range m.Options {
		lines = append(lines, renderMultiselectOption(option, i == m.Index, m.SelectedValue[option.Value]))
	}
	if m.Hint != "" {
		lines = append(lines, RenderMuted(m.Hint))
	}
	if m.Err != "" {
		lines = append(lines, RenderError(m.Err))
	}
	return strings.Join(lines, "\n")
}

type ConfirmModel struct {
	Label     string
	Message   string
	Err       string
	Selected  int
	Cancelled bool
}

func NewConfirmModel(label string, message string, defaultYes bool) *ConfirmModel {
	selected := 1
	if defaultYes {
		selected = 0
	}
	return &ConfirmModel{
		Label:    label,
		Message:  message,
		Selected: selected,
	}
}

func (m *ConfirmModel) YesSelected() bool {
	return m.Selected == 0
}

func (m *ConfirmModel) Init() tea.Cmd { return nil }

func (m *ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c", "q":
		m.Cancelled = true
		return m, tea.Quit
	case "left", "h", "y", "up", "k":
		m.Selected = 0
	case "right", "l", "n", "down", "j":
		m.Selected = 1
	case "enter", " ":
		return m, tea.Quit
	}
	return m, nil
}

func (m *ConfirmModel) View() string {
	yes := confirmInactiveStyle.Render("○ Yes")
	no := confirmInactiveStyle.Render("○ No")
	if m.YesSelected() {
		yes = confirmActiveStyle.Render("● Yes")
	} else {
		no = confirmActiveStyle.Render("● No")
	}
	lines := []string{
		RenderPromptFrame(m.Label, m.Message),
		yes + " / " + no,
	}
	if m.Err != "" {
		lines = append(lines, RenderError(m.Err))
	}
	return strings.Join(lines, "\n")
}

func RenderIntro(tag string, title string, subtitle string) string {
	return RenderTag(tag) + "\n\n" + RenderPrimary(title) + "\n" + RenderMuted(subtitle)
}

func RenderCompletedPrompt(entry TranscriptEntry) string {
	return strings.Join([]string{
		RenderMuted(entry.Message),
		RenderAnswer(entry.Answer),
	}, "\n")
}

func RenderSpinnerPrompt(label string, message string, progress []string) string {
	lines := []string{
		RenderPromptFrame(label, message),
		spinnerStyle.Render("◌ Working..."),
	}
	for _, line := range progress {
		lines = append(lines, RenderMuted("• "+line))
	}
	return strings.Join(lines, "\n")
}

func RenderPromptFrame(label string, message string) string {
	return RenderPromptFrameBlock(label, []string{RenderPrimary(message)})
}

func RenderPromptFrameBlock(label string, bodyLines []string) string {
	tag := RenderTag(label)
	rail := railStyle.Render(renderRail(len(bodyLines)))
	body := strings.Join(bodyLines, "\n")
	return tag + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, rail+"  ", body)
}

func renderRail(height int) string {
	if height <= 1 {
		return "◆"
	}
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		switch i {
		case 1:
			lines = append(lines, "◆")
		default:
			lines = append(lines, "│")
		}
	}
	return strings.Join(lines, "\n")
}

func RenderTag(text string) string     { return tagStyle.Render(text) }
func RenderPrimary(text string) string { return promptStyle.Render(text) }
func RenderMuted(text string) string   { return mutedStyle.Render(text) }
func RenderAnswer(text string) string  { return answerStyle.Render(text) }
func RenderError(text string) string   { return errorStyle.Render(text) }

func renderStatus(text string, icon string) string {
	switch icon {
	case "✓":
		return successStyle.Render(text)
	case "x":
		return errorStyle.Render(text)
	default:
		return mutedStyle.Render(text)
	}
}

func renderSelectOption(option SelectOption, active bool) string {
	prefix := "○ "
	titleStyle := optionStyle
	descStyle := mutedStyle
	if active {
		prefix = activeBulletStyle.Render("● ")
		titleStyle = activeOptionStyle
		descStyle = descriptionStyle
	}
	return prefix + titleStyle.Render(option.Title) + " " + descStyle.Render("("+option.Description+")")
}

func renderMultiselectOption(option SelectOption, active bool, checked bool) string {
	marker := "○"
	if checked {
		marker = "◉"
	}
	markerStyle := optionStyle
	titleStyle := optionStyle
	descStyle := mutedStyle
	if checked {
		markerStyle = activeBulletStyle
	}
	if active {
		titleStyle = activeOptionStyle
		descStyle = descriptionStyle
	}
	return markerStyle.Render(marker) + " " + titleStyle.Render(option.Title) + " " + descStyle.Render("("+option.Description+")")
}

var (
	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0B1220")).
			Background(lipgloss.Color("#22D3EE")).
			Padding(0, 1)
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC"))
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8A8F98"))
	descriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7C818A"))
	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))
	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC"))
	railStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22D3EE"))
	optionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B7BCC4"))
	activeOptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F8FAFC"))
	activeBulletStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22D3EE"))
	confirmActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F8FAFC"))
	confirmInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8A8F98"))
	answerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22D3EE"))
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ADE80"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F87171"))
)
