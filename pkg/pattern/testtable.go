package pattern

// TestTable represents test results with status and timing.
type TestTable struct {
	Label   string
	Results []TestTableItem
}

// TestTableItem is a single test/package result.
type TestTableItem struct {
	Name     string // test or package name
	Status   string // "pass", "fail", "skip"
	Duration string // formatted duration
	Count    int    // number of tests (package-level)
	Details  string // error message or extra info
}

func (t *TestTable) Type() PatternType { return PatternTypeTestTable }
