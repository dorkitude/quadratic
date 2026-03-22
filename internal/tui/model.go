package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"quadratic/internal/config"
	"quadratic/internal/store"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type loginFunc func(context.Context) (string, error)
type syncFunc func(context.Context) (*store.SyncResult, error)

type Model struct {
	ctx     context.Context
	cfg     *config.Config
	store   *store.Store
	summary *store.Summary
	login   loginFunc
	sync    syncFunc
	status  string
	err     error
	busy    bool
	width   int
	height  int
}

type syncDoneMsg struct {
	result *store.SyncResult
	err    error
}

type loginDoneMsg struct {
	token string
	err   error
}

func NewModel(ctx context.Context, cfg *config.Config, _ any, st *store.Store, summary *store.Summary) Model {
	return Model{
		ctx:     ctx,
		cfg:     cfg,
		store:   st,
		summary: summary,
		status:  "Press l to log in, s to sync, q to quit.",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) WithActions(login loginFunc, sync syncFunc) Model {
	m.login = login
	m.sync = sync
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "l":
			if m.busy || m.login == nil {
				return m, nil
			}
			m.busy = true
			m.status = "Waiting for OAuth callback..."
			return m, func() tea.Msg {
				token, err := m.login(m.ctx)
				return loginDoneMsg{token: token, err: err}
			}
		case "s":
			if m.busy || m.sync == nil {
				return m, nil
			}
			m.busy = true
			m.status = "Syncing check-ins..."
			return m, func() tea.Msg {
				result, err := m.sync(m.ctx)
				return syncDoneMsg{result: result, err: err}
			}
		}
	case loginDoneMsg:
		m.busy = false
		m.err = msg.err
		if msg.err == nil {
			m.summary.TokenPresent = msg.token != ""
			m.status = "Token saved."
		}
	case syncDoneMsg:
		m.busy = false
		m.err = msg.err
		if msg.err == nil {
			m.summary.Stored += msg.result.Stored
			m.summary.LastSyncAt = msg.result.FinishedAt
			m.status = fmt.Sprintf("Sync complete. Stored %d new check-ins.", msg.result.Stored)
		}
	}
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("quadratic")
	label := lipgloss.NewStyle().Bold(true)
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	lines := []string{
		title,
		"",
		fmt.Sprintf("%s %s", label.Render("Config:"), value.Render(m.cfg.ConfigPath)),
		fmt.Sprintf("%s %s", label.Render("Data dir:"), value.Render(m.summary.DataDir)),
		fmt.Sprintf("%s %s", label.Render("Token:"), value.Render(boolLabel(m.summary.TokenPresent))),
		fmt.Sprintf("%s %s", label.Render("Stored:"), value.Render(fmt.Sprintf("%d", m.summary.Stored))),
		fmt.Sprintf("%s %s", label.Render("Last sync:"), value.Render(timeLabel(m.summary.LastSyncAt))),
		"",
		m.status,
	}
	if m.err != nil {
		lines = append(lines, "Error: "+m.err.Error())
	}
	lines = append(lines, "", "Keys: l=login s=sync q=quit")

	box := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		Width(max(60, min(m.width-4, 100)))

	return box.Render(strings.Join(lines, "\n"))
}

func boolLabel(value bool) string {
	if value {
		return "present"
	}
	return "missing"
}

func timeLabel(value time.Time) string {
	if value.IsZero() {
		return "never"
	}
	return value.Local().Format(time.RFC3339)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
