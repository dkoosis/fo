package view

import (
	"fmt"

	"github.com/dkoosis/fo/pkg/theme"
)

// DefaultWidth is used when the caller passes width <= 0.
const DefaultWidth = 80

// Render paints a ViewSpec to a string using the supplied theme. Width
// is the available terminal column count; variants that need it
// (Leaderboard bars, SmallMultiples grid) consume it, others ignore.
// Width <= 0 falls back to DefaultWidth.
//
// The type switch is the closed-set check: adding a variant means
// adding a case here. An unknown variant returns a placeholder marker
// rather than panicking, to keep the renderer fail-soft for callers
// composing dynamic specs.
func Render(spec ViewSpec, t theme.Theme, width int) string {
	if width <= 0 {
		width = DefaultWidth
	}
	switch v := spec.(type) {
	case Clean:
		return renderClean(v, t)
	case Bullet:
		return renderBullet(v, t)
	case Grouped:
		return renderGrouped(v, t)
	case Leaderboard:
		return renderLeaderboard(v, t, width)
	case Headline:
		return renderHeadline(v, t)
	case Alert:
		return renderAlert(v, t)
	case Delta:
		return renderDelta(v, t, width)
	case SmallMultiples:
		return renderSmallMultiples(v, t, width)
	default:
		return fmt.Sprintf("<unknown view: %T>", spec)
	}
}
