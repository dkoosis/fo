package design

import (
	"path/filepath"
	"regexp"
	"strings"
)

// PatternMatcher provides intelligent pattern detection for commands and output.
type PatternMatcher struct {
	Config *Config
}

// NewPatternMatcher creates a pattern matcher with the given configuration.
func NewPatternMatcher(config *Config) *PatternMatcher {
	return &PatternMatcher{
		Config: config,
	}
}

// DetectCommandIntent identifies the purpose of a command.
func (pm *PatternMatcher) DetectCommandIntent(cmd string, args []string) string {
	// Check tool-specific configuration first
	if toolConfig := pm.findToolConfig(cmd, args); toolConfig != nil && toolConfig.Intent != "" {
		return toolConfig.Intent
	}

	// Check command against pattern dictionary
	cmdName := filepath.Base(cmd)
	cmdLine := cmdName + " " + strings.Join(args, " ")

	for intent, patterns := range pm.Config.Patterns.Intent {
		for _, pattern := range patterns {
			if strings.Contains(cmdLine, pattern) {
				return intent
			}
		}
	}

	// Check for common action verbs in command name
	commonVerbs := map[string]string{
		"build":   "building",
		"test":    "testing",
		"check":   "checking",
		"lint":    "linting",
		"run":     "running",
		"install": "installing",
		"format":  "formatting",
		"clean":   "cleaning",
		"fetch":   "fetching",
		"pull":    "pulling",
		"push":    "pushing",
		"deploy":  "deploying",
	}

	cmdLower := strings.ToLower(cmdName)
	for verb, intent := range commonVerbs {
		if strings.HasPrefix(cmdLower, verb) || strings.HasSuffix(cmdLower, verb) {
			return intent
		}
	}

	// Check arguments for clues
	for _, arg := range args {
		argLower := strings.ToLower(arg)
		for verb, intent := range commonVerbs {
			if strings.Contains(argLower, verb) {
				return intent
			}
		}
	}

	// Default to "running" if we can't determine intent
	return "running"
}

// ClassifyOutputLine determines the type of an output line
func (pm *PatternMatcher) ClassifyOutputLine(line, cmd string, args []string) (string, LineContext) {
	// Default context
	context := LineContext{
		CognitiveLoad: pm.Config.CognitiveLoad.Default,
		Importance:    2, // Default importance
		IsHighlighted: false,
	}

	// Check tool-specific patterns first
	toolConfig := pm.findToolConfig(cmd, args)
	if toolConfig != nil && toolConfig.OutputPatterns != nil {
		for category, patterns := range toolConfig.OutputPatterns {
			for _, pattern := range patterns {
				// Handle empty pattern as a special case
				if pattern == "" {
					continue
				}

				if regexp.MustCompile(pattern).MatchString(line) {
					return adjustCategoryImportance(category, &context)
				}
			}
		}
	}

	// Check against global patterns
	for category, patterns := range pm.Config.Patterns.Output {
		for _, pattern := range patterns {
			if regexp.MustCompile(pattern).MatchString(line) {
				return adjustCategoryImportance(category, &context)
			}
		}
	}

	// Special case handling for common patterns not covered above

	// Stack traces often have file:line format
	if regexp.MustCompile(`\w+\.(go|js|py|java|rb|cpp|c):\d+`).MatchString(line) {
		context.Importance = 4
		return TypeError, context
	}

	// Lines with "PASS" or "FAIL" for tests
	if regexp.MustCompile(`^(PASS|FAIL):`).MatchString(line) {
		if strings.HasPrefix(line, "PASS") {
			context.Importance = 3
			return TypeSuccess, context
		} else {
			context.Importance = 4
			return TypeError, context
		}
	}

	// Default to detail type with medium importance
	return TypeDetail, context
}

// findToolConfig looks for a configuration for the specific tool being executed
func (pm *PatternMatcher) findToolConfig(cmd string, args []string) *ToolConfig {
	// Get the base command name without path
	cmdName := filepath.Base(cmd)

	// Check for exact command match
	if config, exists := pm.Config.Tools[cmdName]; exists {
		return config
	}

	// Check for command with first arg (e.g., "go test")
	if len(args) > 0 {
		cmdWithArg := cmdName + " " + args[0]
		if config, exists := pm.Config.Tools[cmdWithArg]; exists {
			return config
		}
	}

	return nil
}

// adjustCategoryImportance sets the appropriate importance level based on category
// and returns the category type along with the updated context
func adjustCategoryImportance(category string, context *LineContext) (string, LineContext) {
	switch category {
	case TypeError:
		context.Importance = 5
		context.CognitiveLoad = LoadHigh
		return TypeError, *context
	case TypeWarning:
		context.Importance = 4
		context.CognitiveLoad = LoadMedium
		return TypeWarning, *context
	case TypeSuccess:
		context.Importance = 3
		return TypeSuccess, *context
	case TypeInfo:
		context.Importance = 3
		return TypeInfo, *context
	case TypeProgress:
		context.Importance = 2
		return TypeProgress, *context
	case TypeSummary:
		context.Importance = 4
		context.IsSummary = true
		return TypeSummary, *context
	default:
		// Default to detail with medium importance
		return TypeDetail, *context
	}
}

// FindSimilarLines groups similar output lines for summarization
func (pm *PatternMatcher) FindSimilarLines(lines []OutputLine) map[string][]OutputLine {
	// Group lines by pattern similarity
	groups := make(map[string][]OutputLine)

	for _, line := range lines {
		// Skip lines that are too short to meaningfully group
		if len(line.Content) < 10 {
			key := "short_" + line.Type
			groups[key] = append(groups[key], line)
			continue
		}

		// Extract pattern key from the line
		patternKey := pm.extractPatternKey(line.Content, line.Type)
		groups[patternKey] = append(groups[patternKey], line)
	}

	return groups
}

// extractPatternKey creates a representative key for a line to group similar outputs
func (pm *PatternMatcher) extractPatternKey(content, lineType string) string {
	// Different extraction strategies based on line type
	switch lineType {
	case TypeError, TypeWarning:
		// For errors and warnings, use file:line format as base if present
		fileLineMatch := regexp.MustCompile(`([^/\s]+\.[a-zA-Z0-9]+:\d+)`).FindStringSubmatch(content)
		if len(fileLineMatch) > 1 {
			return lineType + "_" + fileLineMatch[1]
		}

		// Otherwise use first significant word
		words := strings.Fields(content)
		if len(words) > 1 {
			return lineType + "_" + words[0] + "_" + words[1]
		}
	}

	// Default fallback for grouping
	return lineType + "_" + strings.Split(content, " ")[0]
}

// DetermineCognitiveLoad analyzes output to determine overall cognitive load
func (pm *PatternMatcher) DetermineCognitiveLoad(lines []OutputLine) CognitiveLoadContext {
	if !pm.Config.CognitiveLoad.AutoDetect {
		return pm.Config.CognitiveLoad.Default
	}

	errorCount := 0
	warningCount := 0
	outputSize := len(lines)

	for _, line := range lines {
		switch line.Type { // Changed from if/else if to switch
		case TypeError:
			errorCount++
		case TypeWarning:
			warningCount++
		}
	}

	// Research-based heuristics (Zhou et al.)
	if errorCount > 5 || outputSize > 100 {
		return LoadHigh
	} else if errorCount > 0 || warningCount > 3 || outputSize > 30 {
		return LoadMedium
	}

	return LoadLow
}
