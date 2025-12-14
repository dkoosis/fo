package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// activeTheme is the compiled theme used by the dashboard.
// Set via SetTheme before calling RunDashboard.
var activeTheme *CompiledTheme

func init() {
	// Initialize with default theme
	activeTheme = DefaultDashboardTheme().Compile()
}

// SetTheme sets the active dashboard theme.
func SetTheme(theme *DashboardTheme) {
	if theme != nil {
		activeTheme = theme.Compile()
	}
}

// RunDashboard launches the interactive dashboard with the active theme.
func RunDashboard(ctx context.Context, specs []TaskSpec) (int, error) {
	return RunDashboardWithTheme(ctx, specs, nil)
}

// RunDashboardWithTheme launches the interactive dashboard with a specific theme.
func RunDashboardWithTheme(ctx context.Context, specs []TaskSpec, theme *DashboardTheme) (int, error) {
	if theme != nil {
		activeTheme = theme.Compile()
	}
	program := tea.NewProgram(newModel(ctx, specs), tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil {
		return 1, err
	}
	return finalModel.(model).exitCode(), nil
}

type model struct {
	ctx      context.Context
	specs    []TaskSpec
	tasks    []*Task
	updates  <-chan TaskUpdate
	selected int
	detail   bool
	viewport viewport.Model
	ready    bool
	done     bool
}

func newModel(ctx context.Context, specs []TaskSpec) model {
	vp := viewport.New(0, 0)
	vp.SetContent("Select a task to view output")
	tasks, updates := StartTasks(ctx, specs)
	return model{ctx: ctx, specs: specs, tasks: tasks, updates: updates, viewport: vp}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.listenUpdates(), tea.Tick(time.Second/8, func(time.Time) tea.Msg { return tickMsg{} }))
}

type tickMsg struct{}
type taskUpdateMsg TaskUpdate
type doneMsg struct{}

func (m model) listenUpdates() tea.Cmd {
	return func() tea.Msg {
		update, ok := <-m.updates
		if !ok {
			return doneMsg{}
		}
		return taskUpdateMsg(update)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.done {
				return m, tea.Quit
			}
		case "esc":
			m.detail = false
		case "enter":
			m.detail = true
			m.refreshViewport()
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				if m.detail {
					m.refreshViewport()
				}
			}
		case "down", "j":
			if m.selected < len(m.tasks)-1 {
				m.selected++
				if m.detail {
					m.refreshViewport()
				}
			}
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 5
		m.ready = true
	case tickMsg:
		return m, tea.Tick(time.Second/8, func(time.Time) tea.Msg { return tickMsg{} })
	case taskUpdateMsg:
		up := TaskUpdate(msg)
		if up.Index < len(m.tasks) {
			task := m.tasks[up.Index]
			if !up.StartedAt.IsZero() && task.StartedAt.IsZero() {
				task.StartedAt = up.StartedAt
			}
			if up.Line != "" {
				task.appendLine(up.Line)
				if m.detail && m.selected == up.Index {
					m.refreshViewport()
				}
			}
			if up.Status == TaskSuccess || up.Status == TaskFailed {
				task.Status = up.Status
				task.ExitCode = up.ExitCode
				task.FinishedAt = up.FinishedAt
			} else {
				task.Status = up.Status
			}
		}
		if m.allDone() {
			m.done = true
		}
		return m, m.listenUpdates()
	case doneMsg:
		m.done = m.allDone()
		return m, nil
	}
	return m, nil
}

func (m *model) refreshViewport() {
	if m.selected < 0 || m.selected >= len(m.tasks) {
		return
	}
	task := m.tasks[m.selected]
	lines := strings.Join(task.Output, "\n")
	header := fmt.Sprintf("%s/%s: %s\n\n", task.Spec.Group, task.Spec.Name, task.Spec.Command)
	m.viewport.SetContent(header + lines)
}

func (m model) View() string {
	if !m.ready {
		return "Loading dashboard..."
	}
	var b strings.Builder

	// Title bar
	titleText := activeTheme.TitleIcon + " " + activeTheme.TitleText
	title := activeTheme.TitleStyle.Render(titleText)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Task list
	b.WriteString(renderList(m.tasks, m.selected, m.viewport.Width))
	b.WriteString("\n")

	// Detail pane or help
	if m.detail {
		task := m.tasks[m.selected]
		header := activeTheme.DetailHeaderStyle.Render(fmt.Sprintf("\U0001F4CB %s/%s", task.Spec.Group, task.Spec.Name))
		cmd := lipgloss.NewStyle().Foreground(activeTheme.MutedColor()).Render("$ " + task.Spec.Command)
		content := header + "\n" + cmd + "\n\n" + m.viewport.View()
		boxWidth := m.viewport.Width - 4
		if boxWidth < 40 {
			boxWidth = 40
		}
		b.WriteString(activeTheme.DetailBoxStyle.Width(boxWidth).Render(content))
	} else {
		help := activeTheme.StatusBarStyle.Render("\u2191/\u2193 navigate \u2022 Enter view details \u2022 Esc back \u2022 q quit")
		b.WriteString(help)
	}
	return b.String()
}

func renderList(tasks []*Task, selected int, width int) string {
	var lines []string
	groupOrder := make([]string, 0)
	grouped := make(map[string][]int)
	for i, task := range tasks {
		if _, ok := grouped[task.Spec.Group]; !ok {
			groupOrder = append(groupOrder, task.Spec.Group)
		}
		grouped[task.Spec.Group] = append(grouped[task.Spec.Group], i)
	}
	for _, g := range groupOrder {
		lines = append(lines, activeTheme.GroupHeaderStyle.Render(activeTheme.Icons.Group+" "+g))
		for _, idx := range grouped[g] {
			task := tasks[idx]
			duration := ""
			if task.Status == TaskRunning || task.Status == TaskSuccess || task.Status == TaskFailed {
				duration = activeTheme.DurationStyle.Render(" " + formatDuration(task.Duration()))
			}
			taskName := fmt.Sprintf("%s %s%s", statusIcon(task), task.Spec.Name, duration)
			if idx == selected {
				line := activeTheme.SelectedStyle.Render(activeTheme.Icons.Select + " " + taskName)
				lines = append(lines, line)
			} else {
				line := activeTheme.UnselectedStyle.Render("  " + taskName)
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")
	boxWidth := width - 4
	if boxWidth < 40 {
		boxWidth = 40
	}
	return activeTheme.TaskListStyle.Width(boxWidth).Render(content)
}

func statusIcon(task *Task) string {
	switch task.Status {
	case TaskPending:
		return activeTheme.PendingIconStyle.Render(activeTheme.Icons.Pending)
	case TaskRunning:
		frames := activeTheme.SpinnerFrames
		interval := time.Duration(activeTheme.SpinnerInterval) * time.Millisecond
		idx := int(time.Since(task.StartedAt)/interval) % len(frames)
		return activeTheme.RunningIconStyle.Render(frames[idx])
	case TaskSuccess:
		return activeTheme.SuccessIconStyle.Render(activeTheme.Icons.Success)
	case TaskFailed:
		return activeTheme.ErrorIconStyle.Render(activeTheme.Icons.Error)
	default:
		return "?"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Truncate(10 * time.Millisecond).String()
}

func (m model) allDone() bool {
	for _, task := range m.tasks {
		if task.Status == TaskPending || task.Status == TaskRunning {
			return false
		}
	}
	return true
}

func (m model) exitCode() int {
	for _, task := range m.tasks {
		if task.Status == TaskFailed {
			return 1
		}
	}
	return 0
}
