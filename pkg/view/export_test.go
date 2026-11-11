package view

import (
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/scene"
	"github.com/dkoosis/fo/pkg/theme"
)

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

// PickView selects a ViewSpec from a Report. Pure and deterministic:
// branches are evaluated in fixed priority order so the same Report
// always yields the same shape. Delta wraps the inner pick when Diff
// is present and at least one severity bucket moved.
func PickView(r report.Report) ViewSpec {
	return PickViewMode(r, ModeHuman)
}

// RenderSceneLLMString renders a Scene to its canonical `# fo:scene`
// text form as a string. Companion to RenderSceneLLM for callers (the
// cast rail) that need the rendered frame as a value. No error is
// returned: the only writes go to a strings.Builder, whose Write never
// fails. Result round-trips through scene.Parse.
func RenderSceneLLMString(s scene.Scene) string {
	var b strings.Builder
	_ = RenderSceneLLM(&b, s)
	return b.String()
}

// RenderReport picks a view from r and writes the rendered string to w.
// Batch arrival mode: caller has the complete Report.
func RenderReport(w io.Writer, r report.Report, t theme.Theme, width int) error {
	return RenderReportMode(w, r, t, width, ModeHuman)
}

// RenderReportMode is RenderReport with an explicit audience mode.
func RenderReportMode(w io.Writer, r report.Report, t theme.Theme, width int, mode Mode) error {
	return RenderReportModeWithExpand(w, r, t, width, mode, expandSet{})
}
