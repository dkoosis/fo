// Package magetasks provides mage build tasks for fo.
//
// This file defines the main build orchestration using fo's section system.
package magetasks

// RunAll executes the comprehensive build and test workflow.
func RunAll() error {
	sections := []Section{
		{
			Name:        "Build",
			Description: "Build the fo binary",
			Run:         BuildAll,
		},
		{
			Name:        "Tests & Quality",
			Description: "Run tests and quality checks",
			Run:         TestReport,
		},
	}

	_, err := RunSections(sections...)
	return err
}
