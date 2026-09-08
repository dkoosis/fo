package sarif

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/dkoosis/fo/pkg/fingerprint"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/score"
)

// ToReport projects a SARIF Document onto the canonical Report shape.
// Tool name is taken from the first run's driver; multi-run documents
// keep all results but the Tool field reflects only the first.
//
// Findings carry per-finding Score and Fingerprint, with Score reflecting
// occurrence count across the whole document so that widespread defects
// rank above isolated ones.
//
// generatedAt, when given and non-zero, stamps Report.GeneratedAt instead
// of the wall clock — mirrors pkg/state/runlog.go's inject-or-fallback
// convention so output is deterministic under test.
func ToReport(doc *Document, generatedAt ...time.Time) *report.Report {
	var seed time.Time
	if len(generatedAt) > 0 {
		seed = generatedAt[0]
	}
	r := &report.Report{
		GeneratedAt: report.GeneratedAtOrNow(seed),
	}
	if len(doc.Runs) > 0 {
		r.Tool = doc.Runs[0].Tool.Driver.Name
	}

	occ := occurrenceCounts(doc)

	for _, run := range doc.Runs {
		for _, res := range run.Results {
			file := ""
			if len(res.Locations) > 0 {
				file = res.Locations[0].PhysicalLocation.ArtifactLocation.URI
			}
			key := res.RuleID + "\x00" + fingerprint.NormalizeMessage(res.Message.Text)
			n := occ[key]
			if n == 0 {
				n = 1
			}
			sev := mapSeverity(res.Level)
			r.Findings = append(r.Findings, report.Finding{
				RuleID:      res.RuleID,
				File:        file,
				Line:        res.Line(),
				Col:         res.Col(),
				Severity:    sev,
				Message:     res.Message.Text,
				FixCommand:  res.FixCommand(),
				Fingerprint: fingerprint.Fingerprint(res.RuleID, file, res.Message.Text),
				Score:       score.Score(score.SeverityWeight(sev), n, file),
			})
		}
	}

	sort.SliceStable(r.Findings, func(i, j int) bool {
		if r.Findings[i].Score != r.Findings[j].Score {
			return r.Findings[i].Score > r.Findings[j].Score
		}
		if r.Findings[i].File != r.Findings[j].File {
			return r.Findings[i].File < r.Findings[j].File
		}
		return r.Findings[i].Line < r.Findings[j].Line
	})

	return r
}

// ToReportWithMeta is ToReport but stamps DataHash from the raw input bytes
// the caller already has, instead of recomputing from the parsed document.
func ToReportWithMeta(doc *Document, rawInput []byte, generatedAt ...time.Time) *report.Report {
	r := ToReport(doc, generatedAt...)
	if len(rawInput) > 0 {
		sum := sha256.Sum256(rawInput)
		r.DataHash = hex.EncodeToString(sum[:])
	}
	return r
}

func occurrenceCounts(doc *Document) map[string]int {
	counts := make(map[string]int)
	for _, run := range doc.Runs {
		for _, res := range run.Results {
			key := res.RuleID + "\x00" + fingerprint.NormalizeMessage(res.Message.Text)
			counts[key]++
		}
	}
	return counts
}

func mapSeverity(level Level) report.Severity {
	switch level {
	case LevelError:
		return report.SeverityError
	case LevelWarning:
		return report.SeverityWarning
	case LevelNote, LevelNone, "":
		return report.SeverityNote
	default:
		return report.SeverityNote
	}
}
