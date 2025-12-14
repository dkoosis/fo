package dashboard

import (
	"strings"
	"testing"
)

func TestParseManifest(t *testing.T) {
	input := `
Build:
  build: go build ./...
Quality:
  lint: golangci-lint run
  vet: go vet ./...
`
	specs, err := ParseManifest(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("expected 3 specs, got %d", len(specs))
	}
	if specs[0].Group != "Build" || specs[0].Name != "build" || specs[0].Command != "go build ./..." {
		t.Fatalf("unexpected first spec: %+v", specs[0])
	}
	if specs[2].Group != "Quality" || specs[2].Name != "vet" {
		t.Fatalf("unexpected grouping order")
	}
}

func TestParseTaskFlag(t *testing.T) {
	spec, err := ParseTaskFlag("Group/name:echo test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Group != "Group" || spec.Name != "name" || spec.Command != "echo test" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
}
