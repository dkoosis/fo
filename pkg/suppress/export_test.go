package suppress

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

// Match returns the index of the first suppression in rs that matches
// (ruleID, path), or -1 if none match.
func (rs *Ruleset) Match(ruleID, path string) int {
	if rs == nil {
		return -1
	}
	for i := range rs.Suppressions {
		if matchSuppression(rs.Suppressions[i], ruleID, path) {
			return i
		}
	}
	return -1
}
