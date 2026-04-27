package state

import (
	"fmt"
	"strings"
)

// Headline returns the single-line headline band for a Diff in the
// "3 new · 2 resolved · 1 regressed · 2 flaky · 41 persistent" form.
// Zero-count segments are dropped to keep the band scannable; an
// empty diff returns "no changes" so the caller never has to special-case.
func Headline(d Diff) string {
	parts := make([]string, 0, 5)
	if n := len(d.New); n > 0 {
		parts = append(parts, fmt.Sprintf("%d new", n))
	}
	if n := len(d.Resolved); n > 0 {
		parts = append(parts, fmt.Sprintf("%d resolved", n))
	}
	if n := len(d.Regressed); n > 0 {
		parts = append(parts, fmt.Sprintf("%d regressed", n))
	}
	if n := len(d.Flaky); n > 0 {
		parts = append(parts, fmt.Sprintf("%d flaky", n))
	}
	if d.PersistentCount > 0 {
		parts = append(parts, fmt.Sprintf("%d persistent", d.PersistentCount))
	}
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, " · ")
}

// Envelope is the JSON-shape for the `diff` block emitted in llm and
// json formats. PersistentCount is exported as an integer rather than
// a slice — the persistent rows can be tens of thousands and rarely
// drive action in an LLM prompt.
type Envelope struct {
	Headline        string `json:"headline"`
	New             []Item `json:"new"`
	Resolved        []Item `json:"resolved"`
	Regressed       []Item `json:"regressed"`
	Flaky           []Item `json:"flaky"`
	PersistentCount int    `json:"persistent_count"`
}

// EnvelopeOf builds the Envelope from a Diff, ready for JSON marshal.
func EnvelopeOf(d Diff) Envelope {
	return Envelope{
		Headline:        Headline(d),
		New:             nonNil(d.New),
		Resolved:        nonNil(d.Resolved),
		Regressed:       nonNil(d.Regressed),
		Flaky:           nonNil(d.Flaky),
		PersistentCount: d.PersistentCount,
	}
}

// nonNil normalizes a nil slice to an empty slice so the JSON output
// is `"new": []` rather than `"new": null`. LLM consumers and shell
// pipelines parse the empty array more reliably than null.
func nonNil(items []Item) []Item {
	if items == nil {
		return []Item{}
	}
	return items
}
