package dashboard

import (
	"context"
	"encoding/json"
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
	ctx          context.Context
	specs        []TaskSpec
	tasks        []*Task
	updates      <-chan TaskUpdate
	selected     int
	visualOrder  []int // maps visual position to task index
	viewport     viewport.Model
	ready        bool
	done         bool
	width        int // terminal width
	height       int // terminal height
	listWidth    int // width allocated to task list
	detailWidth  int // width allocated to detail pane
}

func newModel(ctx context.Context, specs []TaskSpec) model {
	vp := viewport.New(0, 0)
	vp.SetContent("Select a task to view output")
	tasks, updates := StartTasks(ctx, specs)
	visualOrder := buildVisualOrder(tasks)
	return model{ctx: ctx, specs: specs, tasks: tasks, updates: updates, visualOrder: visualOrder, viewport: vp}
}

// buildVisualOrder creates a mapping from visual position to task index.
// Preserves input order - tasks appear in the order they were specified.
func buildVisualOrder(tasks []*Task) []int {
	order := make([]int, len(tasks))
	for i := range tasks {
		order[i] = i
	}
	return order
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
			if m.selected < len(m.visualOrder)-1 {
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
			// Get the currently selected task index
			selectedTaskIdx := -1
			if m.selected >= 0 && m.selected < len(m.visualOrder) {
				selectedTaskIdx = m.visualOrder[m.selected]
			}
			if up.Line != "" {
				task.appendLine(up.Line)
				if selectedTaskIdx == up.Index {
					m.refreshViewport()
				}
			}
			if up.Status == TaskSuccess || up.Status == TaskFailed {
				task.Status = up.Status
				task.ExitCode = up.ExitCode
				task.FinishedAt = up.FinishedAt
				// Refresh viewport on completion so formatter sees complete output
				if selectedTaskIdx == up.Index {
					m.refreshViewport()
				}
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
	if m.selected < 0 || m.selected >= len(m.visualOrder) {
		return
	}
	taskIdx := m.visualOrder[m.selected]
	task := m.tasks[taskIdx]
	// Use formatter to render output based on command type
	// GetOutput returns a thread-safe copy of the output lines
	formatted := FormatOutput(task.Spec.Command, task.GetOutput(), m.detailWidth)
	m.viewport.SetContent(formatted)
}

func (m model) View() string {
	if !m.ready {
		return "Loading dashboard..."
	}

	// Title bar (full width)
	// Note: Leading newline ensures title is visible in terminals that clip the first line (VS Code)
	titleText := activeTheme.TitleText
	if activeTheme.TitleIcon != "" {
		titleText = activeTheme.TitleIcon + " " + titleText
	}
	if titleText == "" {
		titleText = "Dashboard" // Fallback if no title configured
	}
	title := "\n" + activeTheme.TitleStyle.Width(m.width+5).Height(1).Render(titleText)

	// Panel height for content (excluding borders/padding)
	// blank(1) + title(1) + status(2) + box chrome(4) = 8 total
	contentHeight := m.height - 8
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Left panel: Task list
	// Convert visual position to task index for renderList
	selectedTaskIdx := -1
	if m.selected >= 0 && m.selected < len(m.visualOrder) {
		selectedTaskIdx = m.visualOrder[m.selected]
	}
	listContent := renderList(m.tasks, selectedTaskIdx, m.listWidth)
	// Pad or truncate list content to exact height
	listLines := strings.Split(listContent, "\n")
	for len(listLines) < contentHeight {
		listLines = append(listLines, "")
	}
	if len(listLines) > contentHeight {
		listLines = listLines[:contentHeight]
	}
	// Calculate total panel height (content + style chrome)
	// Detail has border(2) + padding(2) = 4 extra lines
	// List has padding(2) = 2 extra lines, needs +2 to match
	panelHeight := contentHeight + 4
	listPanel := activeTheme.TaskListStyle.
		Width(m.listWidth).
		Height(panelHeight).
		Render(strings.Join(listLines, "\n"))

	// Right panel: Detail pane
	var detailContent string
	if m.selected >= 0 && m.selected < len(m.visualOrder) {
		taskIdx := m.visualOrder[m.selected]
		task := m.tasks[taskIdx]
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

	// Join panels with subtle separator
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Render(strings.Repeat("│\n", panelHeight-1) + "│")
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, separator, detailPanel)

	// Status bar
	help := activeTheme.StatusBarStyle.Render("\u2191/\u2193 navigate \u2022 q quit")

	// Use JoinVertical for proper layout
	return lipgloss.JoinVertical(lipgloss.Left, title, panels, help)
}

func renderList(tasks []*Task, selected int, width int) string {
	lines := make([]string, 0, len(tasks)*2) // pre-allocate for tasks + group headers
	lineWidth := width - 6                   // Account for padding
	if lineWidth < 20 {
		lineWidth = 20
	}

	// Render in input order, showing group header when group changes
	lastGroup := ""
	for idx, task := range tasks {
		if task.Spec.Group != lastGroup {
			lines = append(lines, activeTheme.GroupHeaderStyle.Render(activeTheme.Icons.Group+" "+task.Spec.Group))
			lastGroup = task.Spec.Group
		}

		metric := extractQuickMetric(task)
		if idx == selected {
			// Selected: use raw icons/text so SelectedStyle controls all styling
			duration := ""
			if task.Status == TaskRunning || task.Status == TaskSuccess || task.Status == TaskFailed {
				duration = " " + formatDuration(task.Duration())
			}
			content := fmt.Sprintf("%s %s %s%s%s", activeTheme.Icons.Select, rawStatusIcon(task), task.Spec.Name, metric, duration)
			line := activeTheme.SelectedStyle.Width(lineWidth).Render(content)
			lines = append(lines, line)
		} else {
			// Unselected: use styled icons
			duration := ""
			if task.Status == TaskRunning || task.Status == TaskSuccess || task.Status == TaskFailed {
				duration = activeTheme.DurationStyle.Render(" " + formatDuration(task.Duration()))
			}
			styledMetric := ""
			if metric != "" {
				styledMetric = activeTheme.DurationStyle.Render(metric)
			}
			taskName := fmt.Sprintf("%s %s%s%s", statusIcon(task), task.Spec.Name, styledMetric, duration)
			line := activeTheme.UnselectedStyle.Render("  " + taskName)
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// extractQuickMetric returns a brief inline metric for the task list.
// Returns empty string if no meaningful metric can be extracted.
func extractQuickMetric(task *Task) string {
	if task.Status == TaskPending || task.Status == TaskRunning {
		return ""
	}

	cmd := task.Spec.Command
	output := task.GetOutput()

	// Go test: extract coverage
	if strings.Contains(cmd, "go test") && strings.Contains(cmd, "-json") {
		cov := extractTestCoverage(output)
		if cov > 0 {
			return fmt.Sprintf(" %.0f%%", cov)
		}
		return ""
	}

	// Linters: extract error count
	if strings.Contains(cmd, "golangci-lint") || strings.Contains(cmd, "go vet") {
		count := extractLintErrorCount(output)
		if count > 0 {
			return fmt.Sprintf(" %d", count)
		}
		return ""
	}

	// gofmt: count files needing formatting (exits 0 even with issues)
	if strings.Contains(cmd, "gofmt") {
		count := countNonEmptyLines(output)
		if count > 0 {
			return fmt.Sprintf(" %d", count)
		}
		return ""
	}

	return ""
}

// countNonEmptyLines counts non-empty lines (for gofmt file list, etc.)
func countNonEmptyLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// extractTestCoverage parses go test -json output for coverage percentage.
func extractTestCoverage(lines []string) float64 {
	var maxCov float64
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event struct {
			Action string  `json:"Action"`
			Output string  `json:"Output"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Action == "output" && strings.HasPrefix(event.Output, "coverage:") {
			var cov float64
			if _, err := fmt.Sscanf(event.Output, "coverage: %f%%", &cov); err == nil && cov > maxCov {
				maxCov = cov
			}
		}
	}
	return maxCov
}

// extractLintErrorCount counts issues from SARIF or plain lint output.
func extractLintErrorCount(lines []string) int {
	// Try SARIF first
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "{") && strings.Contains(line, `"runs"`) {
			var report struct {
				Runs []struct {
					Results []struct{} `json:"results"`
				} `json:"runs"`
			}
			if err := json.Unmarshal([]byte(line), &report); err == nil {
				count := 0
				for _, run := range report.Runs {
					count += len(run.Results)
				}
				return count
			}
		}
	}

	// Fallback: count non-empty lines (rough heuristic)
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	if count > 50 {
		return 50 // Cap to avoid huge numbers
	}
	return count
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
