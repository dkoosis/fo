package dashboard

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// TaskSpec represents a single task declaration.
type TaskSpec struct {
	Group   string
	Name    string
	Command string
}

// ParseManifest parses a manifest provided via stdin style input.
// Example:
// Group:
//
//	name: command
func ParseManifest(r io.Reader) ([]TaskSpec, error) {
	var specs []TaskSpec
	scanner := bufio.NewScanner(r)
	currentGroup := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			currentGroup = strings.TrimSuffix(line, ":")
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid manifest line: %s", line)
		}
		name := strings.TrimSpace(parts[0])
		cmd := strings.TrimSpace(parts[1])
		if name == "" || cmd == "" {
			return nil, fmt.Errorf("invalid manifest line: %s", line)
		}
		group := currentGroup
		if group == "" {
			group = "Tasks"
		}
		specs = append(specs, TaskSpec{Group: group, Name: name, Command: cmd})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return specs, nil
}

// ParseTaskFlag parses a repeated --task flag with format group/name:command.
func ParseTaskFlag(raw string) (TaskSpec, error) {
	var spec TaskSpec
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return spec, fmt.Errorf("invalid task format (missing command): %s", raw)
	}
	left := parts[0]
	cmd := strings.TrimSpace(parts[1])
	if cmd == "" {
		return spec, fmt.Errorf("invalid task format (empty command): %s", raw)
	}
	group := "Tasks"
	name := strings.TrimSpace(left)
	if strings.Contains(left, "/") {
		gp := strings.SplitN(left, "/", 2)
		group = strings.TrimSpace(gp[0])
		name = strings.TrimSpace(gp[1])
		if name == "" {
			return spec, fmt.Errorf("invalid task format (missing name): %s", raw)
		}
	}
	spec = TaskSpec{Group: group, Name: name, Command: cmd}
	return spec, nil
}
