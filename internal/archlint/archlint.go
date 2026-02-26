package archlint

import "encoding/json"

// Result represents parsed go-arch-lint check output.
type Result struct {
	HasWarnings bool
	Violations  []Violation
	Checks      []Check
}

// Violation records a forbidden dependency between components.
type Violation struct {
	From     string // component name
	To       string // component name
	FileFrom string
	FileTo   string
}

// Check records whether an architecture rule was exercised.
type Check struct {
	ID   string
	Used bool
}

// Parse decodes go-arch-lint --json output into a Result.
func Parse(data []byte) (*Result, error) {
	var raw struct {
		Payload struct {
			ArchHasWarnings  bool `json:"ArchHasWarnings"`
			ArchWarningsDeps []struct {
				ComponentA struct{ Name string } `json:"ComponentA"`
				ComponentB struct{ Name string } `json:"ComponentB"`
				FileA      string                `json:"FileA"`
				FileB      string                `json:"FileB"`
			} `json:"ArchWarningsDeps"`
			Qualities []struct {
				ID   string `json:"ID"`
				Used bool   `json:"Used"`
			} `json:"Qualities"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	r := &Result{HasWarnings: raw.Payload.ArchHasWarnings}
	for _, d := range raw.Payload.ArchWarningsDeps {
		r.Violations = append(r.Violations, Violation{
			From: d.ComponentA.Name, To: d.ComponentB.Name,
			FileFrom: d.FileA, FileTo: d.FileB,
		})
	}
	for _, q := range raw.Payload.Qualities {
		r.Checks = append(r.Checks, Check{ID: q.ID, Used: q.Used})
	}
	return r, nil
}
