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
	defer os.Setenv("EDITOR", originalEditor)

	// Test with EDITOR set
	os.Setenv("EDITOR", "vim")
	editor := NewEditorMode("")
	assert.Equal(t, "vim", editor.GetEditorCommand())

	// Test without EDITOR (should default to vim)
	os.Unsetenv("EDITOR")
	editor2 := NewEditorMode("")
	assert.Equal(t, "vim", editor2.GetEditorCommand())
}

func TestEditorMode_GetEditorCommand(t *testing.T) {
	t.Parallel()

	editor := NewEditorMode("code")
	assert.Equal(t, "code", editor.GetEditorCommand())
}

