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
			name:     "wrapped exec.ErrNotFound",
			err:      errors.New("wrapped: " + exec.ErrNotFound.Error()),
			expected: true, // String matching will catch "executable file not found" in the error message
		},
		{
			name:     "executable file not found",
			err:      errors.New("executable file not found"),
			expected: true,
		},
		{
			name:     "no such file or directory",
			err:      errors.New("no such file or directory"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
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
