package magetasks

import (
	"errors"
	"os/exec"
	"testing"
)

func TestIsCommandNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "exec.ErrNotFound",
			err:      exec.ErrNotFound,
			expected: true,
		},
		{
			name: "wrapped exec.ErrNotFound",
			err:  errors.New("wrapped: " + exec.ErrNotFound.Error()), //nolint:err113 // Test helper needs dynamic error
			// String matching will catch "executable file not found" in the error message
			expected: true,
		},
		{
			name:     "executable file not found",
			err:      errors.New("executable file not found"), //nolint:err113 // Test helper needs dynamic error
			expected: true,
		},
		{
			name:     "no such file or directory",
			err:      errors.New("no such file or directory"), //nolint:err113 // Test helper needs dynamic error
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"), //nolint:err113 // Test helper needs dynamic error
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommandNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("IsCommandNotFound(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
