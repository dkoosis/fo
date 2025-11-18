//go:build mage

package main

import "github.com/davidkoosis/fo/mageconsole"

var console = mageconsole.DefaultConsole()

// Build contains build-related tasks.
type Build struct{}

// All builds the module using the mage console output.
func (Build) All() error {
	_, err := console.Run("Go Build", "go", "build", "./...")
	return err
}

// Test contains test-related tasks.
type Test struct{}

// All runs the test suite using the mage console output.
func (Test) All() error {
	_, err := console.Run("Go Test", "go", "test", "./...")
	return err
}
