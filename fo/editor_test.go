package fo

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEditorMode_When_EditorProvided(t *testing.T) {
	t.Parallel()

	editor := NewEditorMode("nano")
	assert.Equal(t, "nano", editor.GetEditorCommand())
}

func TestNewEditorMode_When_EmptyEditor(t *testing.T) {
	t.Parallel()

	// Save original EDITOR
	originalEditor := os.Getenv("EDITOR")
	defer func() {
		_ = os.Setenv("EDITOR", originalEditor) // Best effort restore
	}()

	// Test with EDITOR set
	if err := os.Setenv("EDITOR", "vim"); err != nil {
		t.Fatalf("Failed to set EDITOR: %v", err)
	}
	editor := NewEditorMode("")
	assert.Equal(t, "vim", editor.GetEditorCommand())

	// Test without EDITOR (should default to vim)
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatalf("Failed to unset EDITOR: %v", err)
	}
	editor2 := NewEditorMode("")
	assert.Equal(t, "vim", editor2.GetEditorCommand())
}

func TestEditorMode_GetEditorCommand(t *testing.T) {
	t.Parallel()

	editor := NewEditorMode("code")
	assert.Equal(t, "code", editor.GetEditorCommand())
}
