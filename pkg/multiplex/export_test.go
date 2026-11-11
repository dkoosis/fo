package multiplex

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

// isDelimiter reports whether line is a valid section delimiter (recognized
// format value).
func isDelimiter(line []byte) bool {
	return delimiterRe.Match(line)
}
