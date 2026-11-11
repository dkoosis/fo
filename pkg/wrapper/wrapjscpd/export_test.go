package wrapjscpd

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

func newJscpd() jscpd { return jscpd{} }
