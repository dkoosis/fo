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
func ToReport(doc *Document) *report.Report {
	r := &report.Report{
		GeneratedAt: time.Now().UTC(),
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
			r.Findings = append(r.Findings, report.Finding{
				RuleID:      res.RuleID,
				File:        file,
				Line:        res.Line(),
				Col:         res.Col(),
				Severity:    mapSeverity(res.Level),
				Message:     res.Message.Text,
				FixCommand:  res.FixCommand(),
				Fingerprint: fingerprint.Fingerprint(res.RuleID, file, res.Message.Text),
				Score:       score.Score(score.SeverityWeight(res.Level), n, file),
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
func ToReportWithMeta(doc *Document, rawInput []byte) *report.Report {
	r := ToReport(doc)
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

func mapSeverity(level string) report.Severity {
	switch level {
	case "error":
		return report.SeverityError
	case "warning":
		return report.SeverityWarning
	case "note", "none", "":
		return report.SeverityNote
	default:
		return report.SeverityNote
	}
}
