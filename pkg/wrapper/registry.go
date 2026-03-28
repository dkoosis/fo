package wrapper

import (
	"fmt"
	"sort"
)

type entry struct {
	wrapper     Wrapper
	description string
}

var registry = map[string]entry{}

// Register adds a wrapper to the global registry under the given name.
// Intended for use in sub-package init() functions.
func Register(name, description string, w Wrapper) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("wrapper: duplicate registration %q", name))
	}
	registry[name] = entry{wrapper: w, description: description}
}

// Get returns the named wrapper, or nil if not found.
func Get(name string) Wrapper {
	e, ok := registry[name]
	if !ok {
		return nil
	}
	return e.wrapper
}

// Description returns the description for the named wrapper, or "" if not found.
func Description(name string) string {
	return registry[name].description
}

// Names returns all registered wrapper names, sorted.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}