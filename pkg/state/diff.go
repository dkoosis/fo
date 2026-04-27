package state

import (
	"sort"

	"github.com/dkoosis/fo/pkg/report"
)

// Class is the diff classification of a single finding versus prior runs.
type Class string

const (
	ClassNew        Class = "new"
	ClassPersistent Class = "persistent"
	ClassResolved   Class = "resolved"
	ClassRegressed  Class = "regressed"
	ClassFlaky      Class = "flaky"
)

// Item is one classified entry in the diff envelope. Resolved entries
// carry the prior fingerprint+severity; new/regressed/persistent carry
// the current snapshot. Flaky entries reference the current finding
// (since a flaky finding is, by definition, present in the current run
// after having been resolved).
type Item struct {
	Fingerprint  string          `json:"fingerprint"`
	RuleID       string          `json:"rule_id,omitempty"`
	File         string          `json:"file,omitempty"`
	Severity     Severity        `json:"severity"`
	PriorSeverity Severity       `json:"prior_severity,omitempty"`
	Class        Class           `json:"class"`
	report       *report.Finding // unexported back-pointer; not serialized
}

// Diff is the classifier output, both the headline summary inputs and
// the structured envelope payload.
type Diff struct {
	New             []Item `json:"new"`
	Resolved        []Item `json:"resolved"`
	Regressed       []Item `json:"regressed"`
	Flaky           []Item `json:"flaky"`
	PersistentCount int    `json:"persistent_count"`

	// Persistent is the full persistent slice, omitted from JSON to keep
	// the envelope compact. Available for renderers that want the rows.
	Persistent []Item `json:"-"`
}

// Classify compares a current report against prior history. prev may be
// nil (no sidecar yet) — in that case every current finding is "new"
// and nothing is resolved or flaky.
//
// Flake detection requires at least two prior runs in history:
// the finding must be present at t-2, absent at t-1, and present at t.
// With fewer history slots flake is unreachable and the classifier
// degrades gracefully to new/persistent/resolved/regressed only.
func Classify(prev *File, current *report.Report) Diff {
	cur := RunFromReport(current)
	var prior Run
	var older []Run
	if prev != nil && len(prev.Runs) > 0 {
		prior = prev.Runs[0]
		if len(prev.Runs) > 1 {
			older = prev.Runs[1:]
		}
	}

	byFP := make(map[string]*report.Finding, len(current.Findings))
	for i := range current.Findings {
		f := &current.Findings[i]
		if f.Fingerprint != "" {
			byFP[f.Fingerprint] = f
		}
	}

	var d Diff

	for fp, sev := range cur.Findings {
		f := byFP[fp]
		priorSev, hadPrior := prior.Findings[fp]
		switch {
		case !hadPrior:
			if isFlaky(fp, older) {
				d.Flaky = append(d.Flaky, makeItem(fp, sev, "", ClassFlaky, f))
				continue
			}
			d.New = append(d.New, makeItem(fp, sev, "", ClassNew, f))
		case severityRank(sev) > severityRank(priorSev):
			d.Regressed = append(d.Regressed, makeItem(fp, sev, priorSev, ClassRegressed, f))
		default:
			d.Persistent = append(d.Persistent, makeItem(fp, sev, priorSev, ClassPersistent, f))
		}
	}

	for fp, sev := range prior.Findings {
		if _, stillThere := cur.Findings[fp]; stillThere {
			continue
		}
		d.Resolved = append(d.Resolved, makeItem(fp, "", sev, ClassResolved, nil))
	}

	d.PersistentCount = len(d.Persistent)

	sortItems(d.New)
	sortItems(d.Resolved)
	sortItems(d.Regressed)
	sortItems(d.Flaky)
	sortItems(d.Persistent)
	return d
}

// isFlaky reports whether a fingerprint absent from the immediate prior
// run was present in any older run. The "resolved-then-new" pattern
// across ≥2 runs is the minimum signal — we don't try to distinguish
// gradations of flakiness here.
func isFlaky(fp string, older []Run) bool {
	for _, r := range older {
		if _, ok := r.Findings[fp]; ok {
			return true
		}
	}
	return false
}

func makeItem(fp string, sev, prior Severity, c Class, f *report.Finding) Item {
	it := Item{
		Fingerprint:   fp,
		Severity:      sev,
		PriorSeverity: prior,
		Class:         c,
		report:        f,
	}
	if f != nil {
		it.RuleID = f.RuleID
		it.File = f.File
	}
	return it
}

func sortItems(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RuleID != items[j].RuleID {
			return items[i].RuleID < items[j].RuleID
		}
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		return items[i].Fingerprint < items[j].Fingerprint
	})
}
