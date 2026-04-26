package score

import "testing"

func TestSeverityWeight_MapsKnownLevels(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"error":   SeverityWeightError,
		"warning": SeverityWeightWarning,
		"note":    SeverityWeightNote,
		"none":    SeverityWeightNote, // unknown falls back to note
		"":        SeverityWeightNote,
	}
	for level, want := range cases {
		if got := SeverityWeight(level); got != want {
			t.Errorf("SeverityWeight(%q) = %d, want %d", level, got, want)
		}
	}
}

func TestFileCentrality_PrecedenceRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path string
		want float64
	}{
		// _test.go beats every other rule, even when nested under pkg/ or cmd/.
		{"test_file_under_pkg_wins_over_root", "pkg/mapper/sarif_test.go", CentralityTest},
		{"test_file_under_cmd_wins_over_root", "cmd/fo/main_test.go", CentralityTest},
		{"test_file_under_internal_wins_over_internal", "internal/detect/detect_test.go", CentralityTest},
		// internal/ next.
		{"internal_dir", "internal/detect/detect.go", CentralityInternal},
		{"internal_nested", "src/internal/foo/bar.go", CentralityInternal},
		// cmd/ and pkg/ get root weight.
		{"cmd_root", "cmd/fo/main.go", CentralityRoot},
		{"pkg_root", "pkg/mapper/sarif.go", CentralityRoot},
		// Anything else defaults to root weight.
		{"other_path", "vendor/foo/bar.go", CentralityDefault},
		// Windows separators normalize.
		{"windows_test_path", `pkg\mapper\sarif_test.go`, CentralityTest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FileCentrality(tc.path); got != tc.want {
				t.Errorf("FileCentrality(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestScore_SeverityOccurrenceCentralityMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		sev    int
		occ    int
		path   string
		want   float64
	}{
		{"error_x1_pkg_root", SeverityWeightError, 1, "pkg/x/x.go", 3.0},
		{"error_x3_pkg_root", SeverityWeightError, 3, "pkg/x/x.go", 9.0},
		{"warning_x2_internal", SeverityWeightWarning, 2, "internal/x/x.go", 2.0},
		{"note_x5_root", SeverityWeightNote, 5, "pkg/x/x.go", 5.0},
		{"error_x4_test_file", SeverityWeightError, 4, "pkg/x/x_test.go", 3.0},
		{"zero_occurrence_zero_score", SeverityWeightError, 0, "pkg/x/x.go", 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Score(tc.sev, tc.occ, tc.path); got != tc.want {
				t.Errorf("Score(%d, %d, %q) = %v, want %v",
					tc.sev, tc.occ, tc.path, got, tc.want)
			}
		})
	}
}

// TestScore_ProductionFailBeatsTestFail verifies the centrality precedence:
// an error in pkg/ root code outranks an error of identical severity and
// occurrence in a test file under the same package.
func TestScore_ProductionFailBeatsTestFail(t *testing.T) {
	t.Parallel()
	prod := Score(SeverityWeightError, 1, "pkg/foo/foo.go")
	test := Score(SeverityWeightError, 1, "pkg/foo/foo_test.go")
	if !(prod > test) {
		t.Fatalf("expected production score (%v) > test score (%v)", prod, test)
	}
}
