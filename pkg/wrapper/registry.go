package wrapper

import "sort"

var registry = map[string]Wrapper{}

// Register adds a wrapper to the global registry under the given name.
// Intended for use in sub-package init() functions.
func Register(name string, w Wrapper) {
	registry[name] = w
}

// Get returns the named wrapper, or nil if not found.
func Get(name string) Wrapper {
	return registry[name]
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
