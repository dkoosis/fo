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
	ctx        context.Context
	specs      []TaskSpec
	tasks      []*Task
	updates    <-chan TaskUpdate
	selected   int
	viewport   viewport.Model
	ready      bool
	done       bool
	width      int // terminal width
	height     int // terminal height
	listWidth  int // width allocated to task list
	detailWidth int // width allocated to detail pane
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
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.refreshViewport()
			}
		case "down", "j":
			if m.selected < len(m.tasks)-1 {
				m.selected++
				m.refreshViewport()
			}
		}
	case tea.WindowSizeMsg:
		// Subtract 7 chars for scrollbar + window chrome in terminals like VS Code
		m.width = msg.Width - 7
		m.height = msg.Height
		// Calculate list width based on content (auto-fit, minimal padding)
		m.listWidth = m.calculateListWidth()
		if m.listWidth < 22 {
			m.listWidth = 22
		}
		if m.listWidth > m.width/2 {
			m.listWidth = m.width / 2
		}
		m.detailWidth = m.width - m.listWidth - 1 // 1 for gap
		m.viewport.Width = m.detailWidth - 4      // account for box padding + border
		m.viewport.Height = msg.Height - 10       // account for title, header, cmd, status bar, borders
		m.ready = true
		m.refreshViewport()
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
				if m.selected == up.Index {
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

func (m *model) calculateListWidth() int {
	maxWidth := 0
	for _, task := range m.tasks {
		// Group header: "▸ GroupName"
		groupLen := len(task.Spec.Group) + 3
		if groupLen > maxWidth {
			maxWidth = groupLen
		}
		// Task line: "▶ ✓ taskname 12.3s"
		taskLen := len(task.Spec.Name) + 14 // icon + status + duration estimate
		if taskLen > maxWidth {
			maxWidth = taskLen
		}
	}
	// Add minimal padding for box borders
	return maxWidth + 4
}

func (m *model) refreshViewport() {
	if m.selected < 0 || m.selected >= len(m.tasks) {
		return
	}
	task := m.tasks[m.selected]
	// Use formatter to render output based on command type
	formatted := FormatOutput(task.Spec.Command, task.Output, m.detailWidth)
	m.viewport.SetContent(formatted)
}

func (m model) View() string {
	if !m.ready {
		return "Loading dashboard..."
	}

	// Title bar (full width)
	// Note: Leading newline ensures title is visible in terminals that clip the first line (VS Code)
	titleText := activeTheme.TitleIcon + " " + activeTheme.TitleText
	if titleText == " " {
		titleText = "Dashboard" // Fallback if no title configured
	}
	title := "\n" + activeTheme.TitleStyle.Width(m.width + 5).Height(1).Render(titleText)

	// Panel height for content (excluding borders/padding)
	// blank(1) + title(1) + status(2) + box chrome(4) = 8 total
	contentHeight := m.height - 8
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Left panel: Task list
	listContent := renderList(m.tasks, m.selected, m.listWidth)
	// Pad or truncate list content to exact height
	listLines := strings.Split(listContent, "\n")
	for len(listLines) < contentHeight {
		listLines = append(listLines, "")
	}
	if len(listLines) > contentHeight {
		listLines = listLines[:contentHeight]
	}
	listPanel := activeTheme.TaskListStyle.
		Width(m.listWidth).
		Render(strings.Join(listLines, "\n"))

	// Right panel: Detail pane
	var detailContent string
	if m.selected >= 0 && m.selected < len(m.tasks) {
		task := m.tasks[m.selected]
		header := activeTheme.DetailHeaderStyle.Render(fmt.Sprintf("%s/%s", task.Spec.Group, task.Spec.Name))
		detailContent = header + "\n\n" + m.viewport.View()
	} else {
		detailContent = "Select a task to view output"
	}
	// Pad or truncate detail content to exact height
	detailLines := strings.Split(detailContent, "\n")
	for len(detailLines) < contentHeight {
		detailLines = append(detailLines, "")
	}
	if len(detailLines) > contentHeight {
		detailLines = detailLines[:contentHeight]
	}
	detailPanel := activeTheme.DetailBoxStyle.
		Width(m.detailWidth).
		Render(strings.Join(detailLines, "\n"))

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)

	// Status bar
	help := activeTheme.StatusBarStyle.Render("\u2191/\u2193 navigate \u2022 q quit")

	// Use JoinVertical for proper layout
	return lipgloss.JoinVertical(lipgloss.Left, title, panels, help)
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
	lineWidth := width - 6 // Account for padding
	if lineWidth < 20 {
		lineWidth = 20
	}
	for _, g := range groupOrder {
		lines = append(lines, activeTheme.GroupHeaderStyle.Render(activeTheme.Icons.Group+" "+g))
		for _, idx := range grouped[g] {
			task := tasks[idx]
			if idx == selected {
				// Selected: use raw icons/text so SelectedStyle controls all styling
				duration := ""
				if task.Status == TaskRunning || task.Status == TaskSuccess || task.Status == TaskFailed {
					duration = " " + formatDuration(task.Duration())
				}
				content := fmt.Sprintf("%s %s %s%s", activeTheme.Icons.Select, rawStatusIcon(task), task.Spec.Name, duration)
				line := activeTheme.SelectedStyle.Width(lineWidth).Render(content)
				lines = append(lines, line)
			} else {
				// Unselected: use styled icons
				duration := ""
				if task.Status == TaskRunning || task.Status == TaskSuccess || task.Status == TaskFailed {
					duration = activeTheme.DurationStyle.Render(" " + formatDuration(task.Duration()))
				}
				taskName := fmt.Sprintf("%s %s%s", statusIcon(task), task.Spec.Name, duration)
				line := activeTheme.UnselectedStyle.Render("  " + taskName)
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
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

// rawStatusIcon returns the icon without styling (for use in selected rows).
func rawStatusIcon(task *Task) string {
	switch task.Status {
	case TaskPending:
		return activeTheme.Icons.Pending
	case TaskRunning:
		frames := activeTheme.SpinnerFrames
		interval := time.Duration(activeTheme.SpinnerInterval) * time.Millisecond
		idx := int(time.Since(task.StartedAt)/interval) % len(frames)
		return frames[idx]
	case TaskSuccess:
		return activeTheme.Icons.Success
	case TaskFailed:
		return activeTheme.Icons.Error
	default:
		return "?"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		// Show milliseconds for sub-second durations
		ms := d.Milliseconds()
		return fmt.Sprintf("%dms", ms)
	}
	// Show tenths of a second (e.g., 1.2s not 1.34s)
	tenths := d.Round(100 * time.Millisecond)
	secs := tenths.Seconds()
	return fmt.Sprintf("%.1fs", secs)
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
