package sarif

import (
	"cmp"
	"slices"
)

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
			case LevelError:
				fi.ErrorCount++
			case LevelWarning:
				fi.WarnCount++
			}
		}
	}

	// Convert to slice and sort by issue count descending
	files := make([]FileIssue, 0, len(byFile))
	for _, fi := range byFile {
		files = append(files, *fi)
	}
	slices.SortFunc(files, func(a, b FileIssue) int {
		return cmp.Compare(b.IssueCount, a.IssueCount)
	})

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
