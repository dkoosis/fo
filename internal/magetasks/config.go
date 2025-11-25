package magetasks

import (
	"os"
	"path/filepath"
)

var (
	// ModulePath is the Go module path.
	ModulePath = "github.com/dkoosis/fo"

	// BinPath is the output path for built binaries.
	BinPath = "./bin/fo"

	// ProjectRoot is the root directory of the project.
	ProjectRoot string
)

// Initialize sets up the magetasks package.
// Call this from the Magefile init() function.
func Initialize() error {
	var err error
	ProjectRoot, err = os.Getwd()
	if err != nil {
		return err
	}

	// Ensure bin directory exists
	binDir := filepath.Join(ProjectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return err
	}

	return nil
}
