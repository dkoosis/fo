package magetasks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LintAll runs all linters.
func LintAll() error {
	PrintH2Header("Linting")

	var errs []error

	// Go format
	if err := LintFormat(); err != nil {
		errs = append(errs, err)
	}

	// Go vet
	if err := LintVet(); err != nil {
		errs = append(errs, err)
	}

	// Staticcheck (optional)
	if err := LintStaticcheck(); err != nil {
		if !isCommandNotFound(err) {
			errs = append(errs, err)
		}
	}

	// Golangci-lint (optional)
	if err := LintGolangci(); err != nil {
		if !isCommandNotFound(err) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		PrintError("Linting found issues")
		return errors.Join(errs...)
	}

	PrintSuccess("All linters passed")
	return nil
}

// LintFormat checks code formatting.
func LintFormat() error {
	fmt.Println("Running go fmt...")
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("format check failed: %w", err)
	}
	return nil
}

// LintVet runs go vet.
func LintVet() error {
	fmt.Println("Running go vet...")
	cmd := exec.Command("go", "vet", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vet failed: %w", err)
	}
	return nil
}

// LintStaticcheck runs staticcheck.
func LintStaticcheck() error {
	fmt.Println("Running staticcheck...")
	cmd := exec.Command("staticcheck", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if isCommandNotFound(err) {
			PrintWarning("Staticcheck not found (install: go install honnef.co/go/tools/cmd/staticcheck@latest)")
			return err
		}
		return fmt.Errorf("staticcheck failed: %w", err)
	}
	return nil
}

// LintGolangci runs golangci-lint.
func LintGolangci() error {
	fmt.Println("Running golangci-lint...")
	cmd := exec.Command("golangci-lint", "run",
		"--enable-all",
		"--disable=exhaustruct,varnamelen,ireturn,wrapcheck,nlreturn,gochecknoglobals,mnd,depguard,tagalign,tenv",
		"--timeout=5m",
		"./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if isCommandNotFound(err) {
			PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
			return err
		}
		return fmt.Errorf("golangci-lint failed: %w", err)
	}
	return nil
}

// LintGolangciFix runs golangci-lint with auto-fixes.
func LintGolangciFix() error {
	fmt.Println("Running golangci-lint with auto-fix...")
	cmd := exec.Command("golangci-lint", "run", "--fix",
		"--enable-all",
		"--disable=exhaustruct,varnamelen,ireturn,wrapcheck,nlreturn,gochecknoglobals,mnd,depguard,tagalign,tenv",
		"--timeout=5m",
		"./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if isCommandNotFound(err) {
			PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
			return err
		}
		return fmt.Errorf("golangci-lint failed: %w", err)
	}
	return nil
}

// isCommandNotFound checks if the error indicates the command was not found.
func isCommandNotFound(err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	// Fallback string matching for edge cases
	errStr := err.Error()
	if strings.Contains(errStr, "executable file not found") {
		return true
	}
	if strings.Contains(errStr, "no such file or directory") {
		return true
	}
	return false
}
