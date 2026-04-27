// Package view defines the eight ViewSpec variants that sit between the
// canonical report.Report and the paint primitives. A ViewSpec carries
// the data its variant needs and is rendered by Render. Selection logic
// (pickView) lives in a separate package.
//
// ViewSpec is a closed sum-type: only this package can satisfy the
// unexported isViewSpec marker. Adding a ninth variant means adding a
// case to render.go's type switch — by design.
package view

import (
	"github.com/dkoosis/fo/pkg/report"
)

// ViewSpec is the closed sum of render variants. The isViewSpec marker
// is unexported so external packages cannot extend the set; the type
// switch in Render is the canonical exhaustive handler.
type ViewSpec interface {
	isViewSpec()
}

// Clean — the empty-state view. count == 0 case.
type Clean struct {
	// Message is the affirming line (e.g. "no findings"). Required.
	Message string
}

func (Clean) isViewSpec() {}

// Bullet — flat list of rows, glyph + label + value per row. Used when
// the count is small and items don't share a meaningful grouping.
//
// FixCommand on a row, when non-empty, is rendered as a copy-pastable
// suggestion line beneath the row.
type Bullet struct {
	Items []BulletItem
}

func (Bullet) isViewSpec() {}

// BulletItem is one row in a Bullet or Grouped view.
type BulletItem struct {
	Severity   report.Severity // optional — drives glyph + color
	Outcome    report.TestOutcome
	Label      string
	Value      string // free-form right-side detail (e.g. file:line)
	FixCommand string // optional copy-pastable suggestion
}

// Grouped — flat list with section labels. Used when count is larger
// and items naturally cluster (e.g. by severity or package). Sections
// render in the order given; empty sections are skipped.
type Grouped struct {
	Sections []GroupedSection
}

func (Grouped) isViewSpec() {}

// GroupedSection is a labeled cluster of BulletItems.
type GroupedSection struct {
	Label string
	Items []BulletItem
}

// Leaderboard — ranked items with bars. Used when a small head holds a
// large fraction of the total impact. Total scales every bar.
type Leaderboard struct {
	Rows  []LbRow
	Total float64 // value used to scale bars; rows sum to <= Total
}

func (Leaderboard) isViewSpec() {}

// LbRow is one ranked entry in a Leaderboard.
type LbRow struct {
	Label string
	Value float64
}

// Headline — single dominant message in heading typography. Used for
// PANIC, build-error-only runs, or one overwhelming signal.
type Headline struct {
	Title  string
	Detail string // optional sub-line, rendered muted under Title
}

func (Headline) isViewSpec() {}

// Alert — colored bold prefix + value, single line. Used when one
// metric breaches a threshold and demands attention without dominating.
type Alert struct {
	Severity report.Severity // drives the prefix color
	Prefix   string          // short label, e.g. "ERRORS"
	Value    string          // the metric, e.g. "47"
	Detail   string          // optional muted suffix
}

func (Alert) isViewSpec() {}

// Delta wraps another view with a strip of arrow buckets summarising
// change versus a prior run. Inner is rendered first, then the bucket
// strip below.
//
// By convention Inner is one of Bullet, Leaderboard, or SmallMultiples
// — the three views where row-level deltas carry meaning. The type
// system does not enforce this; pickView is responsible for picking a
// sane Inner.
type Delta struct {
	Inner   ViewSpec
	Buckets []DeltaBucket
}

func (Delta) isViewSpec() {}

// DeltaBucket is one comparison cell: a labeled count with direction
// vs prior. Direction values: +1 up, -1 down, 0 same.
type DeltaBucket struct {
	Label     string
	Count     int
	Direction int
}

// SmallMultiples — Swiss grid of repeated mini-bullets. Used for
// per-package or per-module summaries where the same shape repeats.
// Cells are laid out in a column-aligned grid; whitespace alone makes
// the grid visible.
type SmallMultiples struct {
	Cells []MultipleCell
}

func (SmallMultiples) isViewSpec() {}

// MultipleCell is one tile in a SmallMultiples grid.
type MultipleCell struct {
	Label    string
	Sparks   []float64 // optional sparkline series
	Counters []Counter // 0..n labeled counts, rendered inline
}

// Counter is a labeled small integer used inside a MultipleCell.
type Counter struct {
	Severity report.Severity // optional — drives color
	Label    string
	Value    int
}
