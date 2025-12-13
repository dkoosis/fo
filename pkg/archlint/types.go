// Package archlint provides parsing and rendering for go-arch-lint JSON output.
package archlint

// Result represents the JSON output from `go-arch-lint check --json`.
type Result struct {
	Type    string  `json:"Type"`
	Payload Payload `json:"Payload"`
}

// Payload contains the analysis results.
type Payload struct {
	ExecutionWarnings      []string         `json:"ExecutionWarnings"`
	ArchHasWarnings        bool             `json:"ArchHasWarnings"`
	ArchWarningsDeps       []DepWarning     `json:"ArchWarningsDeps"`
	ArchWarningsNotMatched []string         `json:"ArchWarningsNotMatched"`
	ArchWarningsDeepScan   []DeepScanWarn   `json:"ArchWarningsDeepScan"`
	OmittedCount           int              `json:"OmittedCount"`
	ModuleName             string           `json:"ModuleName"`
	Qualities              []QualityLinter  `json:"Qualities"`
}

// DepWarning represents a dependency violation.
type DepWarning struct {
	ComponentFrom string   `json:"ComponentFrom"`
	ComponentTo   string   `json:"ComponentTo"`
	FileFrom      string   `json:"FileFrom"`
	FileTo        string   `json:"FileTo"`
	Reference     Reference `json:"Reference"`
}

// Reference provides location details for a violation.
type Reference struct {
	Line   int    `json:"Line"`
	Column int    `json:"Column"`
	Name   string `json:"Name"`
}

// DeepScanWarn represents a deep scan warning (method calls, DI).
type DeepScanWarn struct {
	ComponentFrom string    `json:"ComponentFrom"`
	ComponentTo   string    `json:"ComponentTo"`
	FileFrom      string    `json:"FileFrom"`
	Reference     Reference `json:"Reference"`
}

// QualityLinter describes a quality check configuration.
type QualityLinter struct {
	ID   string `json:"ID"`
	Used bool   `json:"Used"`
}

// Stats aggregates analysis statistics.
type Stats struct {
	TotalViolations   int
	DepViolations     int
	NotMatchedFiles   int
	DeepScanWarnings  int
	OmittedCount      int
	ByComponent       map[string]int // violations per source component
	ByTargetComponent map[string]int // violations per target component
}

// ComputeStats calculates summary statistics from a Result.
func ComputeStats(r *Result) Stats {
	s := Stats{
		ByComponent:       make(map[string]int),
		ByTargetComponent: make(map[string]int),
	}

	s.DepViolations = len(r.Payload.ArchWarningsDeps)
	s.NotMatchedFiles = len(r.Payload.ArchWarningsNotMatched)
	s.DeepScanWarnings = len(r.Payload.ArchWarningsDeepScan)
	s.OmittedCount = r.Payload.OmittedCount
	s.TotalViolations = s.DepViolations + s.NotMatchedFiles + s.DeepScanWarnings

	for _, w := range r.Payload.ArchWarningsDeps {
		s.ByComponent[w.ComponentFrom]++
		s.ByTargetComponent[w.ComponentTo]++
	}

	for _, w := range r.Payload.ArchWarningsDeepScan {
		s.ByComponent[w.ComponentFrom]++
		s.ByTargetComponent[w.ComponentTo]++
	}

	return s
}
