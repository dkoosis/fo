package state

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

// FullLogPath returns the resolved path for the tee'd full-output log.
// Single most-recent file, matching last-run.json's "one snapshot" shape
// rather than run-log.json's append history — the log exists to make the
// immediately preceding run's unfiltered output one command away, not to
// build a history.
func FullLogPath() string { return newStateStore().fullLogPath() }
