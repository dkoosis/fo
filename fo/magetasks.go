// fo/magetasks.go - Standard mage task helpers for common Go project operations.
//
// This file provides ready-to-use mage tasks that work out of the box with
// beautiful, themed output. Import this in your magefile to get instant
// professional output for common operations like deps, build, test, lint.
package fo

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ================================
// DEPENDENCY TASKS
// ================================

// DepsUpdate updates all Go dependencies to latest versions.
// Returns a summary message suitable for section display.
func (c *Console) DepsUpdate() (string, error) {
	output, err := c.Run("Update dependencies", "go", "get", "-u", "./...")
	if err != nil {
		return "", err
	}

	if _, err := c.Run("Tidy modules", "go", "mod", "tidy"); err != nil {
		return "", err
	}

	// Analyze output to generate summary
	summary := summarizeGoGet(output)
	return summary, nil
}

// DepsVerify verifies dependencies haven't been modified.
func (c *Console) DepsVerify() error {
	_, err := c.Run("Verify dependencies", "go", "mod", "verify")
	return err
}

// DepsTidy cleans up go.mod and go.sum.
func (c *Console) DepsTidy() error {
	_, err := c.Run("Tidy modules", "go", "mod", "tidy")
	return err
}

func summarizeGoGet(result *TaskResult) string {
	if result == nil || len(result.Lines) == 0 {
		return "Dependencies up to date"
	}

	var added, upgraded int
	for _, line := range result.Lines {
		content := line.Content
		if strings.HasPrefix(content, "go: added ") {
			added++
		} else if strings.HasPrefix(content, "go: upgraded ") {
			upgraded++
		}
	}

	if added == 0 && upgraded == 0 {
		return "Dependencies up to date"
	}

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if upgraded > 0 {
		parts = append(parts, fmt.Sprintf("%d upgraded", upgraded))
	}

	return "Dependencies: " + strings.Join(parts, ", ")
}

// ================================
// BUILD TASKS
// ================================

// BuildGo builds a Go binary with the given output name.
func (c *Console) BuildGo(outputName string, pkg string, ldflags ...string) error {
	args := []string{"build"}

	if len(ldflags) > 0 {
		args = append(args, "-ldflags", strings.Join(ldflags, " "))
	}

	args = append(args, "-o", outputName, pkg)

	_, err := c.Run("Build "+outputName, "go", args...)
	return err
}

// ================================
// TEST TASKS
// ================================

// TestGo runs Go tests with optional coverage.
func (c *Console) TestGo(coverageFile string, packages ...string) (*TestSummary, error) {
	args := []string{"test", "-v"}

	if coverageFile != "" {
		args = append(args, "-coverprofile="+coverageFile, "-covermode=atomic")
	}

	if len(packages) == 0 {
		args = append(args, "./...")
	} else {
		args = append(args, packages...)
	}

	result, err := c.Run("Run tests", "go", args...)
	if err != nil {
		return nil, err
	}

	// Parse test output for summary
	summary := parseTestOutput(result)
	return summary, nil
}

// TestSummary contains aggregated test results.
type TestSummary struct {
	Passed   int
	Failed   int
	Skipped  int
	Total    int
	Coverage float64
}

func parseTestOutput(result *TaskResult) *TestSummary {
	summary := &TestSummary{}
	if result == nil {
		return summary
	}

	for _, line := range result.Lines {
		content := line.Content
		if strings.Contains(content, "--- PASS:") {
			summary.Passed++
		} else if strings.Contains(content, "--- FAIL:") {
			summary.Failed++
		} else if strings.Contains(content, "--- SKIP:") {
			summary.Skipped++
		}
	}

	summary.Total = summary.Passed + summary.Failed + summary.Skipped
	return summary
}

// ================================
// LINT TASKS
// ================================

// LintGo runs golangci-lint on the codebase.
func (c *Console) LintGo() error {
	_, err := c.Run("Run golangci-lint", "golangci-lint", "run", "./...")
	return err
}

// LintVet runs go vet on the codebase.
func (c *Console) LintVet() error {
	_, err := c.Run("Run go vet", "go", "vet", "./...")
	return err
}

// LintStaticcheck runs staticcheck on the codebase.
func (c *Console) LintStaticcheck() error {
	_, err := c.Run("Run staticcheck", "staticcheck", "./...")
	return err
}

// LintFormat checks if code is properly formatted.
func (c *Console) LintFormat() error {
	result, err := c.Run("Check formatting", "gofmt", "-l", ".")
	if err != nil {
		return err
	}

	// gofmt -l returns files that need formatting
	for _, line := range result.Lines {
		if strings.TrimSpace(line.Content) != "" {
			return fmt.Errorf("files need formatting: run 'go fmt ./...'")
		}
	}

	return nil
}

// ================================
// FILE SIZE ANALYSIS
// ================================

// FileSizeConfig configures the file size analysis.
type FileSizeConfig struct {
	WarnLineCount      int    // Warn when regular files exceed this (default: 500)
	ErrorLineCount     int    // Error when regular files exceed this (default: 1000)
	WarnLineCountTest  int    // Warn when test files exceed this (default: 800)
	ErrorLineCountTest int    // Error when test files exceed this (default: 1400)
	TopFilesCount      int    // Show top N largest files (default: 5)
	WarnMarkdownCount  int    // Warn when markdown file count exceeds (default: 50)
	SnapshotDir        string // Directory for storing snapshots (default: ".fo")
}

// DefaultFileSizeConfig returns sensible defaults.
func DefaultFileSizeConfig() FileSizeConfig {
	return FileSizeConfig{
		WarnLineCount:      500,
		ErrorLineCount:     1000,
		WarnLineCountTest:  800,
		ErrorLineCountTest: 1400,
		TopFilesCount:      5,
		WarnMarkdownCount:  50,
		SnapshotDir:        ".fo",
	}
}

// FileInfo holds information about a file's size.
type FileInfo struct {
	Path      string
	LineCount int
	IsTest    bool
}

// FileSizeAnalysis holds the results of file size analysis.
type FileSizeAnalysis struct {
	Files         []FileInfo
	NonTestFiles  []FileInfo
	Warnings      []string
	Errors        []string
	MarkdownCount int
	TotalFiles    int
	Over500Count  int
	Over750Count  int
	Over1000Count int
	GreenCount    int  // Files < 500 LOC
	YellowCount   int  // Files 500-999 LOC
	RedCount      int  // Files >= 1000 LOC
}

// AnalyzeFileSizes performs file size analysis on the repository.
func (c *Console) AnalyzeFileSizes(root string, cfg FileSizeConfig) (*FileSizeAnalysis, error) {
	analysis := &FileSizeAnalysis{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Skip directories we don't care about
		if d.IsDir() {
			base := filepath.Base(relPath)
			if strings.HasPrefix(base, ".") && relPath != "." {
				return fs.SkipDir
			}
			if base == "vendor" || base == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}

		// Count markdown files
		if strings.HasSuffix(d.Name(), ".md") {
			analysis.MarkdownCount++
			return nil
		}

		// Only analyze .go files
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		// Skip generated files
		if strings.Contains(relPath, "vendor/") || strings.Contains(relPath, ".git/") {
			return nil
		}

		lineCount, err := countFileLines(path)
		if err != nil {
			return fmt.Errorf("count lines in %s: %w", relPath, err)
		}

		isTest := strings.HasSuffix(relPath, "_test.go")
		fileInfo := FileInfo{Path: relPath, LineCount: lineCount, IsTest: isTest}
		analysis.Files = append(analysis.Files, fileInfo)

		if !isTest {
			analysis.NonTestFiles = append(analysis.NonTestFiles, fileInfo)

			// Categorize by size
			switch {
			case lineCount >= 1000:
				analysis.RedCount++
				analysis.Over1000Count++
				analysis.Over750Count++
				analysis.Over500Count++
			case lineCount >= 750:
				analysis.YellowCount++
				analysis.Over750Count++
				analysis.Over500Count++
			case lineCount >= 500:
				analysis.YellowCount++
				analysis.Over500Count++
			default:
				analysis.GreenCount++
			}
		}

		// Check thresholds
		warnThreshold := cfg.WarnLineCount
		errorThreshold := cfg.ErrorLineCount
		if isTest {
			warnThreshold = cfg.WarnLineCountTest
			errorThreshold = cfg.ErrorLineCountTest
		}

		if lineCount >= errorThreshold {
			analysis.Errors = append(analysis.Errors, fmt.Sprintf("%d  %s", lineCount, relPath))
		} else if lineCount >= warnThreshold {
			analysis.Warnings = append(analysis.Warnings, fmt.Sprintf("%d  %s", lineCount, relPath))
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk repository: %w", err)
	}

	analysis.TotalFiles = len(analysis.NonTestFiles)

	// Sort by line count descending
	sort.Slice(analysis.Files, func(i, j int) bool {
		return analysis.Files[i].LineCount > analysis.Files[j].LineCount
	})
	sort.Slice(analysis.NonTestFiles, func(i, j int) bool {
		return analysis.NonTestFiles[i].LineCount > analysis.NonTestFiles[j].LineCount
	})

	return analysis, nil
}

// RenderFileSizeReport renders the file size analysis inside a section box.
func (c *Console) RenderFileSizeReport(analysis *FileSizeAnalysis, cfg FileSizeConfig) {
	// File Metrics section
	c.PrintBulletHeader("File Metrics (Non-Test Files)")

	c.PrintMetricLine(MetricLine{Label: "Total files", Value: fmt.Sprintf("%d", analysis.TotalFiles)})
	c.PrintMetricLine(MetricLine{Label: ">500 LOC count", Value: fmt.Sprintf("%d", analysis.Over500Count)})
	c.PrintMetricLine(MetricLine{Label: ">750 LOC count", Value: fmt.Sprintf("%d", analysis.Over750Count)})
	c.PrintMetricLine(MetricLine{Label: ">1000 LOC count", Value: fmt.Sprintf("%d", analysis.Over1000Count)})

	c.PrintBlankLine()

	// Top N largest files
	topCount := cfg.TopFilesCount
	if topCount > len(analysis.NonTestFiles) {
		topCount = len(analysis.NonTestFiles)
	}

	items := make([]RankedItem, topCount)
	for i := 0; i < topCount; i++ {
		f := analysis.NonTestFiles[i]
		items[i] = RankedItem{
			Rank:  i + 1,
			Value: fmt.Sprintf("%d", f.LineCount),
			Label: c.FormatPath(f.Path),
		}
	}

	c.PrintRankedList(fmt.Sprintf("Top %d Largest Non-Test Files", cfg.TopFilesCount), items)

	// Markdown count
	c.PrintBlankLine()
	mdColor := ""
	mdNote := ""
	if analysis.MarkdownCount >= cfg.WarnMarkdownCount {
		mdColor = "warning"
		mdNote = fmt.Sprintf(" (limit: %d)", cfg.WarnMarkdownCount)
	}
	c.PrintMetricLine(MetricLine{
		Label: "Markdown Files",
		Value: fmt.Sprintf("%d%s", analysis.MarkdownCount, mdNote),
		Color: mdColor,
	})
}

func countFileLines(path string) (int, error) {
	file, err := os.Open(path) // #nosec G304 - file paths come from WalkDir
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}

	return count, scanner.Err()
}

