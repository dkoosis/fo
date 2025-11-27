package magetasks

import (
	"errors"
	"fmt"
)

// LintAll runs all linters.
func LintAll() error {
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
		if !IsCommandNotFound(err) {
			errs = append(errs, err)
		}
	}

	// Golangci-lint (optional)
	if err := LintGolangci(); err != nil {
		if !IsCommandNotFound(err) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	SetSectionSummary("All linters passed")
	return nil
}

// LintFormat checks code formatting.
func LintFormat() error {
	return Run("Go Format", "go", "fmt", "./...")
}

// LintVet runs go vet.
func LintVet() error {
	return Run("Go Vet", "go", "vet", "./...")
}

// LintStaticcheck runs staticcheck.
func LintStaticcheck() error {
	if err := Run("Staticcheck", "staticcheck", "./..."); err != nil {
		if IsCommandNotFound(err) {
			PrintWarning("Staticcheck not found (install: go install honnef.co/go/tools/cmd/staticcheck@latest)")
			return err
		}
		return fmt.Errorf("staticcheck failed: %w", err)
	}
	return nil
}

// LintGolangci runs golangci-lint.
func LintGolangci() error {
	if err := Run("Golangci-lint", "golangci-lint", "run",
		"--disable=exhaustruct,varnamelen,ireturn,wrapcheck,nlreturn,gochecknoglobals,mnd,depguard,tagalign,tenv",
		"--timeout=5m",
		"./..."); err != nil {
		if IsCommandNotFound(err) {
			PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
			return err
		}
		return fmt.Errorf("golangci-lint failed: %w", err)
	}
	return nil
}

// LintGolangciFix runs golangci-lint with auto-fixes.
func LintGolangciFix() error {
	if err := Run("Golangci-lint Fix", "golangci-lint", "run", "--fix",
		"--disable=exhaustruct,varnamelen,ireturn,wrapcheck,nlreturn,gochecknoglobals,mnd,depguard,tagalign,tenv",
		"--timeout=5m",
		"./..."); err != nil {
		if IsCommandNotFound(err) {
			PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
			return err
		}
		return fmt.Errorf("golangci-lint failed: %w", err)
	}
	return nil
}
