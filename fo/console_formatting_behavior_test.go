package fo

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestConsole_StylesStatusText_When_UsingStatusCategories(t *testing.T) {
	t.Parallel()

	console := DefaultConsole()

	tests := []struct {
		name          string
		status        string
		expectedColor string
	}{
		{
			name:          "success status uses success color",
			status:        "pass",
			expectedColor: string(console.designConf.GetColor("Success")),
		},
		{
			name:          "failure status uses error color",
			status:        "fail",
			expectedColor: string(console.designConf.GetColor("Error")),
		},
		{
			name:          "warning status uses warning color",
			status:        "warning",
			expectedColor: string(console.designConf.GetColor("Warning")),
		},
		{
			name:          "unknown status falls back to muted color",
			status:        "flaky",
			expectedColor: console.GetMutedColor(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			styled := console.FormatStatusText("check", tc.status)
			expected := lipgloss.NewStyle().Foreground(lipgloss.Color(tc.expectedColor)).Render("check")

			assert.Equal(t, expected, styled)
		})
	}
}

func TestConsole_FormatsTestName_When_CombiningIconAndHumanizedName(t *testing.T) {
	t.Parallel()

	console := DefaultConsole()

	tests := []struct {
		name     string
		status   string
		iconKey  string
		colorKey string
		rawName  string
	}{
		{
			name:     "renders success icon and text for pass status",
			status:   "pass",
			iconKey:  "Success",
			colorKey: "Success",
			rawName:  "TestConsole_Formats_When_NameHasWhen",
		},
		{
			name:     "renders error styling for failure status",
			status:   "fail",
			iconKey:  "Error",
			colorKey: "Error",
			rawName:  "TestConsole_Renders_When_StatusFails",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			icon := console.designConf.GetIcon(tc.iconKey)
			if icon == "" && tc.iconKey == "Success" {
				icon = defaultSuccessIcon
			}

			styledIcon := lipgloss.NewStyle().Foreground(console.designConf.GetColor(tc.colorKey)).Render(icon)
			humanName := HumanizeTestName(tc.rawName)
			styledText := lipgloss.NewStyle().Foreground(console.designConf.GetColor(tc.colorKey)).Render(humanName)

			expected := styledIcon + " " + styledText

			assert.Equal(t, expected, console.FormatTestName(tc.rawName, tc.status))
		})
	}
}

func TestConsole_FormatsPath_When_GivenAbsoluteOrRelativePaths(t *testing.T) {
	t.Parallel()

	console := DefaultConsole()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "empty path returns empty string",
			path:     "",
			expected: "",
		},
		{
			name:     "filename without directory uses white color",
			path:     "file.txt",
			expected: console.GetColor("White") + "file.txt" + string(console.designConf.ResetColor()),
		},
		{
			name:     "directory uses muted color and filename uses white",
			path:     "dir/file.txt",
			expected: console.GetMutedColor() + "dir/" + string(console.designConf.ResetColor()) + console.GetColor("White") + "file.txt" + string(console.designConf.ResetColor()),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, console.FormatPath(tc.path))
		})
	}
}
