package design

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdjustCategoryImportance_UpdatesContext_When_CategoryProvided(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category string
		expected LineContext
	}{
		{
			name:     "elevates errors to high importance",
			category: TypeError,
			expected: LineContext{Importance: 5, CognitiveLoad: LoadHigh},
		},
		{
			name:     "marks warnings with medium load",
			category: TypeWarning,
			expected: LineContext{Importance: 4, CognitiveLoad: LoadMedium},
		},
		{
			name:     "sets summary flag for summaries",
			category: TypeSummary,
			expected: LineContext{Importance: 4, IsSummary: true},
		},
		{
			name:     "defaults detail when category unknown",
			category: "mystery",
			expected: LineContext{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := &LineContext{}
			category, updated := adjustCategoryImportance(tc.category, ctx)

			if tc.expected.IsSummary {
				assert.True(t, updated.IsSummary)
			}
			assert.Equal(t, tc.expected.Importance, updated.Importance)
			assert.Equal(t, tc.expected.CognitiveLoad, updated.CognitiveLoad)
			if tc.category == "mystery" {
				assert.Equal(t, TypeDetail, category)
			} else {
				assert.Equal(t, tc.category, category)
			}
		})
	}
}

func TestPatternMatcher_ExtractsPatternKey_When_ContentContainsFileReference(t *testing.T) {
	t.Parallel()

	pm := NewPatternMatcher(DefaultConfig())

	tests := []struct {
		name     string
		content  string
		lineType string
		expected string
	}{
		{
			name:     "uses file line for errors",
			content:  "main.go:12: failed to compile",
			lineType: TypeError,
			expected: TypeError + "_main.go:12",
		},
		{
			name:     "falls back to first words when no file present",
			content:  "warning: disk nearly full",
			lineType: TypeWarning,
			expected: TypeWarning + "_warning:_disk",
		},
		{
			name:     "returns prefix for info messages",
			content:  "info system ready",
			lineType: TypeInfo,
			expected: TypeInfo + "_info",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, pm.extractPatternKey(tc.content, tc.lineType))
		})
	}
}
