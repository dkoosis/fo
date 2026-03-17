package archlint

import (
	"encoding/json"
	"fmt"
)

// Result represents parsed go-arch-lint check output.
type Result struct {
	HasWarnings bool
	Violations  []Violation
	CheckCount  int
}

// Violation records a forbidden dependency between components.
type Violation struct {
	From     string // component name
	To       string // resolved import name (disallowed dependency)
	FileFrom string // source file relative path
}

// Parse decodes go-arch-lint --json output into a Result.
func Parse(data []byte) (*Result, error) {
	var raw struct {
		Payload struct {
			ArchHasWarnings  bool `json:"ArchHasWarnings"`
			ArchWarningsDeps []struct {
				ComponentName      string `json:"ComponentName"`
				FileRelativePath   string `json:"FileRelativePath"`
				ResolvedImportName string `json:"ResolvedImportName"`
			} `json:"ArchWarningsDeps"`
			Qualities []struct {
				ID   string `json:"ID"`
				Used bool   `json:"Used"`
			} `json:"Qualities"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("archlint: %w", err)
	}

	r := &Result{HasWarnings: raw.Payload.ArchHasWarnings}
	for _, d := range raw.Payload.ArchWarningsDeps {
		r.Violations = append(r.Violations, Violation{
			From:     d.ComponentName,
			To:       d.ResolvedImportName,
			FileFrom: d.FileRelativePath,
		})
	}
	r.CheckCount = len(raw.Payload.Qualities)
	return r, nil
}
