package sarif_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
)

func TestResultLineAndCol_ReturnsZero_When_ResultHasNoLocations(t *testing.T) {
	t.Parallel()

	var r sarif.Result
	assertInt(t, r.Line(), 0, "line without locations")
	assertInt(t, r.Col(), 0, "col without locations")
}

func TestResultLineAndCol_ReturnsPrimaryLocationCoordinates_When_MultipleLocations(t *testing.T) {
	t.Parallel()

	r := sarif.Result{
		Locations: []sarif.Location{
			location("first.go", 33, 7),
			location("second.go", 90, 1),
		},
	}

	assertInt(t, r.Line(), 33, "line from primary location")
	assertInt(t, r.Col(), 7, "col from primary location")
}

func location(file string, line, col int) sarif.Location {
	return sarif.Location{PhysicalLocation: sarif.PhysicalLocation{
		ArtifactLocation: sarif.ArtifactLocation{URI: file},
		Region:           sarif.Region{StartLine: line, StartColumn: col},
	}}
}

func assertInt(t *testing.T, got, want int, field string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %d want %d", field, got, want)
	}
}
