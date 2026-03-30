package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/models"
	"golang.org/x/term"
)

type tailKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Follow   key.Binding
	Quit     key.Binding
}

func newTailKeyMap() tailKeyMap {
	return tailKeyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "space"), key.WithHelp("pgdn", "page down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		Follow:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "follow")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k tailKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Bottom, k.Follow, k.Quit}
}

func (k tailKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.PageUp, k.PageDown}, {k.Top, k.Bottom, k.Follow, k.Quit}}
}

type logsTailMsg struct {
	resp *models.LogsResult
	err  error
}

type logsTailModel struct {
	client      api.Client
	flags       observabilityFlags
	query       string
	info        string
	details     string
	cursor      string
	lines       []string
	follow      bool
	status      string
	err         error
	viewport    viewport.Model
	help        help.Model
	keys        tailKeyMap
	width       int
	height      int
	ready       bool
	polling     bool
	headerStyle lipgloss.Style
	metaStyle   lipgloss.Style
	liveStyle   lipgloss.Style
	statusStyle lipgloss.Style
	bodyStyle   lipgloss.Style
	helpStyle   lipgloss.Style
	frameStyle  lipgloss.Style
}

func newLogsTailModel(client api.Client, query string, flags observabilityFlags) logsTailModel {
	keys := newTailKeyMap()
	helpModel := help.New()
	helpModel.ShowAll = false

	return logsTailModel{
		client:  client,
		flags:   flags,
		query:   query,
		info:    renderTUIInfo(flags),
		details: renderTUIDetails(query, flags),
		follow:  true,
		status:  "connecting",
		help:    helpModel,
		keys:    keys,
		frameStyle: lipgloss.NewStyle().
			Padding(0, 1),
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 1),
		liveStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#081C15")).
			Background(lipgloss.Color("#A7F3D0")).
			Padding(0, 1),
		metaStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CBD5E1")).
			Background(lipgloss.Color("#111827")).
			Padding(0, 1),
		statusStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.Color("#1E293B")).
			Padding(0, 1),
		bodyStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Padding(0, 1),
		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 1),
	}
}

func (m logsTailModel) Init() tea.Cmd {
	return m.pollCmd(0)
}

func (m logsTailModel) pollCmd(delay time.Duration) tea.Cmd {
	req := m.flags.request(m.query, m.cursor, false)
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		resp, err := m.client.TailLogs(req)
		return logsTailMsg{resp: resp, err: err}
	}
}

func (m logsTailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := m.headerHeight(msg.Width)
		footerHeight := m.footerHeight(msg.Width)
		bodyHeight := msg.Height - headerHeight - footerHeight
		if bodyHeight < 1 {
			bodyHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(maxInt(1, msg.Width-2), bodyHeight)
			m.ready = true
		} else {
			m.viewport.Width = maxInt(1, msg.Width-2)
			m.viewport.Height = bodyHeight
		}
		m.refreshViewport()
		return m, nil

	case logsTailMsg:
		m.polling = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "retrying after error"
			return m, m.pollCmd(logsTailPollInterval)
		}
		m.err = nil
		if msg.resp.NextCursor != nil && strings.TrimSpace(*msg.resp.NextCursor) != "" {
			m.cursor = *msg.resp.NextCursor
		}
		newLines := formatLogEntriesForTUI(msg.resp.Data)
		if len(newLines) > 0 {
			m.lines = append(m.lines, newLines...)
			if len(m.lines) > 4000 {
				m.lines = append([]string(nil), m.lines[len(m.lines)-4000:]...)
			}
		}
		if msg.resp.HasMore {
			m.status = fmt.Sprintf("draining %d new lines", len(newLines))
		} else if len(newLines) > 0 {
			m.status = fmt.Sprintf("streaming %d new lines", len(newLines))
		} else {
			m.status = "waiting for new logs"
		}
		m.refreshViewport()
		if msg.resp.HasMore {
			return m, m.pollCmd(0)
		}
		return m, m.pollCmd(logsTailPollInterval)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.ready {
				m.viewport.LineUp(1)
				m.follow = m.atBottom()
			}
		case key.Matches(msg, m.keys.Down):
			if m.ready {
				m.viewport.LineDown(1)
				m.follow = m.atBottom()
			}
		case key.Matches(msg, m.keys.PageUp):
			if m.ready {
				m.viewport.HalfViewUp()
				m.follow = false
			}
		case key.Matches(msg, m.keys.PageDown):
			if m.ready {
				m.viewport.HalfViewDown()
				m.follow = m.atBottom()
			}
		case key.Matches(msg, m.keys.Top):
			if m.ready {
				m.viewport.GotoTop()
				m.follow = false
			}
		case key.Matches(msg, m.keys.Bottom):
			if m.ready {
				m.follow = true
				m.viewport.GotoBottom()
			}
		case key.Matches(msg, m.keys.Follow):
			m.follow = !m.follow
			if m.follow && m.ready {
				m.viewport.GotoBottom()
			}
		}
		return m, nil
	}

	return m, nil
}

func (m logsTailModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m logsTailModel) renderHeader() string {
	title := lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.headerStyle.Render("opc logs tail"),
		" ",
		m.liveStyle.Render("LIVE"),
	)
	top := lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", m.metaStyle.Render(emptyFallback(m.info, "tailing all services")))

	statusParts := []string{
		"status=" + m.status,
		fmt.Sprintf("lines=%d", len(m.lines)),
	}
	if m.follow {
		statusParts = append(statusParts, "follow=on")
	} else {
		statusParts = append(statusParts, "follow=off")
	}
	if strings.TrimSpace(m.cursor) != "" {
		statusParts = append(statusParts, "cursor=active")
	} else {
		statusParts = append(statusParts, "cursor=none")
	}
	if strings.TrimSpace(m.details) != "" {
		statusParts = append(statusParts, m.details)
	}
	if m.err != nil {
		statusParts = append(statusParts, "error="+m.err.Error())
	}

	statusLine := m.statusStyle.Render(strings.Join(statusParts, "  |  "))
	return lipgloss.JoinVertical(lipgloss.Left, top, statusLine)
}

func (m logsTailModel) renderBody() string {
	if !m.ready {
		return m.bodyStyle.Width(m.width).Render("Connecting to log stream...")
	}
	return m.bodyStyle.Width(m.width).Render(m.viewport.View())
}

func (m logsTailModel) renderFooter() string {
	shortHelp := m.help.ShortHelpView(m.keys.ShortHelp())
	return m.helpStyle.Width(m.width).Render(shortHelp)
}

func (m logsTailModel) headerHeight(width int) int {
	return lipgloss.Height(lipgloss.NewStyle().Width(maxInt(1, width)).Render(m.renderHeader()))
}

func (m logsTailModel) footerHeight(width int) int {
	return lipgloss.Height(lipgloss.NewStyle().Width(maxInt(1, width)).Render(m.renderFooter()))
}

func (m *logsTailModel) refreshViewport() {
	if !m.ready {
		return
	}
	previousOffset := m.viewport.YOffset
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	if m.follow {
		m.viewport.GotoBottom()
		return
	}
	m.viewport.YOffset = previousOffset
}

func (m logsTailModel) atBottom() bool {
	if !m.ready {
		return true
	}
	return m.viewport.AtBottom()
}

func runLogsTailTea(ctx context.Context, client api.Client, query string, flags observabilityFlags) error {
	model := newLogsTailModel(client, query, flags)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := program.Run()
	return err
}

func canUseBubbleTailTUI() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}

func formatLogEntriesForTUI(entries []models.LogEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, formatLogEntryForTUI(entry))
	}
	return lines
}

func formatLogEntryForTUI(entry models.LogEntry) string {
	severity := strings.ToUpper(strings.TrimSpace(entry.SeverityText))
	severityBlock := severityStyle(severity).Render(padSeverity(valueOrDash(severity)))
	return fmt.Sprintf(
		"%s  %s  %s  %s",
		mutedStyle().Render(formatTimestamp(entry.Timestamp)),
		severityBlock,
		serviceStyle().Render(valueOrDash(entry.ServiceName)),
		quoteField(entry.Body),
	)
}

func severityStyle(severity string) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	switch severity {
	case "ERROR", "FATAL":
		return style.Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color("#B91C1C"))
	case "WARN":
		return style.Foreground(lipgloss.Color("#111827")).Background(lipgloss.Color("#FDE68A"))
	case "INFO":
		return style.Foreground(lipgloss.Color("#082F49")).Background(lipgloss.Color("#BAE6FD"))
	case "DEBUG", "TRACE":
		return style.Foreground(lipgloss.Color("#E5E7EB")).Background(lipgloss.Color("#475569"))
	default:
		return style.Foreground(lipgloss.Color("#E5E7EB")).Background(lipgloss.Color("#334155"))
	}
}

func mutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
}

func serviceStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0"))
}

func padSeverity(value string) string {
	if len(value) >= 5 {
		return value
	}
	return value + strings.Repeat(" ", 5-len(value))
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
