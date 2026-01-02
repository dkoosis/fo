package design

import (
	"strings"
	"testing"
)

func TestHousekeeping_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess
	cfg.Icons.Warning = IconCharWarning
	cfg.Icons.Error = IconCharError
	cfg.Icons.Info = IconCharBullet
	cfg.Style.Indentation = "  "

	tests := []struct {
		name           string
		housekeeping   *Housekeeping
		wantContain    []string
		wantNotContain []string
	}{
		{
			name: "all checks passing - collapsed view",
			housekeeping: &Housekeeping{
				Title: "HOUSEKEEPING",
				Checks: []HousekeepingCheck{
					{Name: "Markdown files", Status: "pass", Current: 45, Threshold: 50},
					{Name: "TODO comments", Status: "pass", Current: 10, Threshold: 0},
					{Name: "Orphan test files", Status: "pass", Current: 0, Threshold: 0},
				},
			},
			wantContain: []string{
				"HOUSEKEEPING",
				"3/3",
				"✓",
			},
			wantNotContain: []string{
				"Markdown files", // Should be collapsed
			},
		},
		{
			name: "some warnings - expanded view",
			housekeeping: &Housekeeping{
				Title: "HOUSEKEEPING",
				Checks: []HousekeepingCheck{
					{Name: "Markdown files", Status: "warn", Current: 62, Threshold: 50},
					{Name: "TODO comments", Status: "pass", Current: 10, Threshold: 0},
					{Name: "Orphan test files", Status: "pass", Current: 0, Threshold: 0},
				},
			},
			wantContain: []string{
				"HOUSEKEEPING",
				"Markdown files",
				"62 / 50 limit",
				"⚠",
			},
		},
		{
			name: "with details",
			housekeeping: &Housekeeping{
				Title: "HOUSEKEEPING",
				Checks: []HousekeepingCheck{
					{Name: "TODO comments", Status: "warn", Current: 23, Details: "7 older than 90 days"},
				},
			},
			wantContain: []string{
				"TODO comments",
				"23",
				"7 older than 90 days",
			},
		},
		{
			name: "with items",
			housekeeping: &Housekeeping{
				Title: "HOUSEKEEPING",
				Checks: []HousekeepingCheck{
					{
						Name:    "Orphan test files",
						Status:  "warn",
						Current: 2,
						Items:   []string{"pkg/api/orphan_test.go", "pkg/db/stale_test.go"},
					},
				},
			},
			wantContain: []string{
				"Orphan test files",
				"pkg/api/orphan_test.go",
				"pkg/db/stale_test.go",
			},
		},
		{
			name: "items truncation",
			housekeeping: &Housekeeping{
				Title: "HOUSEKEEPING",
				Checks: []HousekeepingCheck{
					{
						Name:    "Dead code",
						Status:  "fail",
						Current: 5,
						Items: []string{
							"file1.go", "file2.go", "file3.go",
							"file4.go", "file5.go",
						},
					},
				},
			},
			wantContain: []string{
				"file1.go",
				"file2.go",
				"file3.go",
				"... and 2 more",
			},
		},
		{
			name: "fail status",
			housekeeping: &Housekeeping{
				Title: "HOUSEKEEPING",
				Checks: []HousekeepingCheck{
					{Name: "Markdown files", Status: "fail", Current: 100, Threshold: 50},
				},
			},
			wantContain: []string{
				"✗",
				"100 / 50 limit",
			},
		},
		{
			name: "empty checks",
			housekeeping: &Housekeeping{
				Title:  "HOUSEKEEPING",
				Checks: []HousekeepingCheck{},
			},
			wantContain: []string{}, // Should be empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.housekeeping.Render(cfg)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("output should contain %q, got: %q", want, got)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("output should NOT contain %q, got: %q", notWant, got)
				}
			}
		})
	}
}

func TestHousekeeping_PatternType(t *testing.T) {
	h := &Housekeeping{}
	got := h.PatternType()
	if got != PatternTypeHousekeeping {
		t.Errorf("PatternType() = %v, want %v", got, PatternTypeHousekeeping)
	}
}

func TestHousekeeping_Counts(t *testing.T) {
	h := &Housekeeping{
		Checks: []HousekeepingCheck{
			{Name: "Check 1", Status: "pass"},
			{Name: "Check 2", Status: "pass"},
			{Name: "Check 3", Status: "warn"},
			{Name: "Check 4", Status: "fail"},
			{Name: "Check 5", Status: "pass"},
		},
	}

	if got := h.PassCount(); got != 3 {
		t.Errorf("PassCount() = %d, want 3", got)
	}

	if got := h.WarnCount(); got != 1 {
		t.Errorf("WarnCount() = %d, want 1", got)
	}

	if got := h.FailCount(); got != 1 {
		t.Errorf("FailCount() = %d, want 1", got)
	}
}

func TestNewHousekeepingCheck(t *testing.T) {
	tests := []struct {
		name       string
		checkName  string
		current    int
		threshold  int
		wantStatus string
	}{
		{"below threshold - pass", "Markdown files", 40, 50, "pass"},
		{"at threshold - pass", "Markdown files", 50, 50, "pass"},
		{"above threshold - warn", "Markdown files", 60, 50, "warn"},
		{"way above threshold - fail", "Markdown files", 80, 50, "fail"},
		{"zero goal, at zero - pass", "Orphan tests", 0, 0, "pass"},
		{"zero goal, small count - warn", "Orphan tests", 2, 0, "warn"},
		{"zero goal, large count - fail", "Orphan tests", 5, 0, "fail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NewHousekeepingCheck(tt.checkName, tt.current, tt.threshold)
			if check.Status != tt.wantStatus {
				t.Errorf("NewHousekeepingCheck(%q, %d, %d).Status = %q, want %q",
					tt.checkName, tt.current, tt.threshold, check.Status, tt.wantStatus)
			}
			if check.Name != tt.checkName {
				t.Errorf("NewHousekeepingCheck().Name = %q, want %q", check.Name, tt.checkName)
			}
			if check.Current != tt.current {
				t.Errorf("NewHousekeepingCheck().Current = %d, want %d", check.Current, tt.current)
			}
			if check.Threshold != tt.threshold {
				t.Errorf("NewHousekeepingCheck().Threshold = %d, want %d", check.Threshold, tt.threshold)
			}
		})
	}
}

func TestHousekeepingCheckType_Constants(t *testing.T) {
	// Verify check type constants are defined
	checkTypes := []HousekeepingCheckType{
		CheckMarkdownCount,
		CheckTodoComments,
		CheckOrphanTests,
		CheckPackageDocs,
		CheckDeadCode,
		CheckDeprecatedDeps,
		CheckLicenseHeaders,
		CheckGeneratedFreshness,
	}

	for _, ct := range checkTypes {
		if ct == "" {
			t.Error("HousekeepingCheckType constant is empty")
		}
	}
}
