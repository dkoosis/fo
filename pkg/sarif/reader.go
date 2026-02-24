package sarif

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Read parses SARIF from an io.Reader.
func Read(r io.Reader) (*Document, error) {
	var doc Document
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode sarif: %w", err)
	}
	return validateDocument(&doc)
}

// ReadBytes parses SARIF from a byte slice.
func ReadBytes(data []byte) (*Document, error) {
	return Read(bytes.NewReader(data))
}

func validateDocument(doc *Document) (*Document, error) {
	if doc.Version == "" {
		return nil, fmt.Errorf("missing sarif version")
	}
	return doc, nil
}

// Stats aggregates statistics from SARIF results.
type Stats struct {
	TotalIssues int
	ByLevel     map[string]int // error, warning, note, none
	ByRule      map[string]int
	ByFile      map[string]int
}

// ComputeStats calculates aggregate statistics from a SARIF document.
func ComputeStats(doc *Document) Stats {
	stats := Stats{
		ByLevel: make(map[string]int),
		ByRule:  make(map[string]int),
		ByFile:  make(map[string]int),
	}

	for _, run := range doc.Runs {
		for _, result := range run.Results {
			stats.TotalIssues++
			stats.ByLevel[result.Level]++
			stats.ByRule[result.RuleID]++

			if len(result.Locations) > 0 {
				file := result.Locations[0].PhysicalLocation.ArtifactLocation.URI
				stats.ByFile[file]++
			}
		}
	}

	return stats
}

// FileIssue represents an issue in a specific file for leaderboard rendering.
type FileIssue struct {
	File       string
	IssueCount int
	ErrorCount int
	WarnCount  int
}

// TopFiles returns files sorted by issue count (descending).
func TopFiles(doc *Document, limit int) []FileIssue {
	byFile := make(map[string]*FileIssue)

	for _, run := range doc.Runs {
		for _, result := range run.Results {
			if len(result.Locations) == 0 {
				continue
			}
			file := result.Locations[0].PhysicalLocation.ArtifactLocation.URI

			fi, ok := byFile[file]
			if !ok {
				fi = &FileIssue{File: file}
				byFile[file] = fi
			}

			fi.IssueCount++
			switch result.Level {
			case "error":
				fi.ErrorCount++
			case "warning":
				fi.WarnCount++
			}
		}
	}

	// Convert to slice and sort
	files := make([]FileIssue, 0, len(byFile))
	for _, fi := range byFile {
		files = append(files, *fi)
	}

	// Sort by issue count descending
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].IssueCount > files[i].IssueCount {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	return files
}

// GroupedResults organizes results by a grouping key.
type GroupedResults struct {
	Key     string   // file path or rule ID
	Results []Result // issues in this group
}

// GroupByFile organizes results by file path.
func GroupByFile(doc *Document) []GroupedResults {
	byFile := make(map[string][]Result)
	var order []string

	for _, run := range doc.Runs {
		for _, result := range run.Results {
			file := "unknown"
			if len(result.Locations) > 0 {
				file = result.Locations[0].PhysicalLocation.ArtifactLocation.URI
			}

			if _, seen := byFile[file]; !seen {
				order = append(order, file)
			}
			byFile[file] = append(byFile[file], result)
		}
	}

	groups := make([]GroupedResults, 0, len(byFile))
	for _, file := range order {
		groups = append(groups, GroupedResults{
			Key:     file,
			Results: byFile[file],
		})
	}

	return groups
}

