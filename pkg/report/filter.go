package report

import (
	"fmt"
	"time"

	"github.com/dkoosis/fo/pkg/suppress"
)

// FilterStats records the outcome of ApplyFilter for callers that want
// per-rule visibility. Total is also reflected on Report.Suppressed.
type FilterStats struct {
	Total   int
	PerRule map[int]int
}

// ApplyFilter removes Findings matched by active (non-expired) rules in
// rs and increments Report.Suppressed. Expired rules that still match
// leave the finding in place and append a Notice warning that the
// suppression is past its until-date (one Notice per expired rule with
// at least one match). A nil/empty ruleset is a no-op.
func ApplyFilter(r *Report, rs *suppress.Ruleset, now time.Time) FilterStats {
	stats := FilterStats{PerRule: map[int]int{}}
	if r == nil || rs == nil || len(rs.Rules) == 0 || len(r.Findings) == 0 {
		return stats
	}

	expiredNotified := map[int]bool{}
	kept := r.Findings[:0]
	for _, f := range r.Findings {
		idx := rs.Match(f.RuleID, f.File)
		if idx < 0 {
			kept = append(kept, f)
			continue
		}
		rule := rs.Rules[idx]
		if rule.Expired(now) {
			kept = append(kept, f)
			if !expiredNotified[idx] {
				expiredNotified[idx] = true
				until := ""
				if rule.Until != nil {
					until = rule.Until.Format("2006-01-02")
				}
				r.Notices = append(r.Notices, fmt.Sprintf(
					"suppression for %s (line %d) expired %s; finding shown",
					rule.RuleID, rule.Line, until))
			}
			continue
		}
		stats.Total++
		stats.PerRule[idx]++
	}
	r.Findings = kept
	r.Suppressed += stats.Total
	return stats
}
