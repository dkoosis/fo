// Package fo provides editor mode for viewing and editing output.
package fo

import (
	"os"
	"os/exec"
	"strings"
)

// EditorMode provides an interface for viewing/editing output in an external editor.
type EditorMode struct {
	editorCmd string // Editor command (e.g., "vim", "nano", "code")
}

// NewEditorMode creates a new EditorMode instance.
// If editor is empty, it will attempt to detect from $EDITOR environment variable.
func NewEditorMode(editor string) *EditorMode {
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Default fallback
		}
	}
	return &EditorMode{
		editorCmd: editor,
	}
}

// OpenInEditor opens the given content in an external editor.
// Returns the edited content and any error that occurred.
func (e *EditorMode) OpenInEditor(content string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "fo-edit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Open in editor
	editorParts := strings.Fields(e.editorCmd)
	cmd := exec.Command(editorParts[0], append(editorParts[1:], tmpFile.Name())...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read edited content
	editedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return string(editedContent), nil
}

// GetEditorCommand returns the editor command being used.
func (e *EditorMode) GetEditorCommand() string {
	return e.editorCmd
}

