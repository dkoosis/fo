package design

import (
	"path/filepath"
	"regexp"
	"strings"
)

// CompiledPattern holds a precompiled regex pattern with metadata.
type CompiledPattern struct {
	Re     *regexp.Regexp
	Type   string
	Weight int
}

// PatternMatcher provides intelligent pattern detection for commands and output.
type PatternMatcher struct {
	Config                 *Config
	compiledOutputPatterns map[string][]CompiledPattern
	compiledToolPatterns   map[string]map[string][]CompiledPattern
	// Precompiled special patterns
	fileLinePattern *regexp.Regexp
	passFailPattern *regexp.Regexp
}

// NewPatternMatcher creates a pattern matcher with the given configuration.
// All regex patterns are precompiled at initialization time for performance.
func NewPatternMatcher(config *Config) *PatternMatcher {
	pm := &PatternMatcher{
		Config:                 config,
		compiledOutputPatterns: make(map[string][]CompiledPattern),
		compiledToolPatterns:   make(map[string]map[string][]CompiledPattern),
	}

	// Precompile global output patterns
	for category, patterns := range config.Patterns.Output {
		compiled := make([]CompiledPattern, 0, len(patterns))
		for _, pattern := range patterns {
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				// Skip invalid patterns, but in production you might want to log this
				continue
			}
			compiled = append(compiled, CompiledPattern{
				Re:     re,
				Type:   category,
				Weight: 1,
			})
		}
		if len(compiled) > 0 {
			pm.compiledOutputPatterns[category] = compiled
		}
	}

	// Precompile tool-specific patterns
	for toolName, toolConfig := range config.Tools {
		if toolConfig == nil || toolConfig.OutputPatterns == nil {
			continue
		}
		toolPatterns := make(map[string][]CompiledPattern)
		for category, patterns := range toolConfig.OutputPatterns {
			compiled := make([]CompiledPattern, 0, len(patterns))
			for _, pattern := range patterns {
				if pattern == "" {
					continue
				}
				re, err := regexp.Compile(pattern)
				if err != nil {
					continue
				}
				compiled = append(compiled, CompiledPattern{
					Re:     re,
					Type:   category,
					Weight: 1,
				})
			}
			if len(compiled) > 0 {
				toolPatterns[category] = compiled
			}
		}
		if len(toolPatterns) > 0 {
			pm.compiledToolPatterns[toolName] = toolPatterns
		}
	}

	// Precompile special patterns used in ClassifyOutputLine
	pm.fileLinePattern = regexp.MustCompile(`\w+\.(go|js|py|java|rb|cpp|c):\d+`)
	pm.passFailPattern = regexp.MustCompile(`^(PASS|FAIL):`)

	return pm
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

// Fast-path prefix checks for common patterns (avoids regex overhead).
var (
	errorPrefixes   = []string{"Error:", "ERROR:", "E!", "panic:", "fatal:", "Failed", "[ERROR]", "FAIL\t"}
	warningPrefixes = []string{"Warning:", "WARNING:", "WARN", "W!", "deprecated:", "[warn]", "[WARNING]", "Warn:"}
	successPrefixes = []string{"Success:", "SUCCESS:", "PASS\t", "ok\t", "Done!", "Completed", "✓", "All tests passed!"}
	infoPrefixes    = []string{"Info:", "INFO:", "INFO[", "I!", "[info]", "Running"}
)

// hasPrefix checks if line starts with any of the given prefixes (case-insensitive for some).
func hasPrefix(line string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
		// Case-insensitive check for common uppercase/lowercase variants
		if len(line) >= len(prefix) {
			lineLower := strings.ToLower(line[:len(prefix)])
			prefixLower := strings.ToLower(prefix)
			if lineLower == prefixLower {
				return true
			}
		}
	}
	return false
}

// ClassifyOutputLine determines the type of an output line.
// Uses fast-path string prefix checks before falling back to regex patterns for performance.
// When multiple patterns match, the category with the highest accumulated score wins.
func (pm *PatternMatcher) ClassifyOutputLine(line, cmd string, args []string) (string, LineContext) {
	// Default context
	context := LineContext{
		CognitiveLoad: pm.Config.CognitiveLoad.Default,
		Importance:    2, // Default importance
		IsHighlighted: false,
	}

	// Fast-path: Check common prefixes before regex (much faster for most cases)
	// These are strong signals with high confidence, so return immediately
	if hasPrefix(line, errorPrefixes) {
		context.Importance = 5
		context.CognitiveLoad = LoadHigh
		return TypeError, context
	}
	if hasPrefix(line, warningPrefixes) {
		context.Importance = 4
		context.CognitiveLoad = LoadMedium
		return TypeWarning, context
	}
	if hasPrefix(line, successPrefixes) {
		context.Importance = 3
		return TypeSuccess, context
	}
	if hasPrefix(line, infoPrefixes) {
		context.Importance = 3
		return TypeInfo, context
	}

	// Scoring-based classification for regex patterns
	// Accumulate scores per category, return the highest
	scores := make(map[string]int)

	// Check tool-specific patterns using precompiled regexes
	toolConfig := pm.findToolConfig(cmd, args)
	if toolConfig != nil {
		cmdName := filepath.Base(cmd)
		toolKey := cmdName
		if len(args) > 0 {
			toolKeyWithArg := cmdName + " " + args[0]
			if patterns, ok := pm.compiledToolPatterns[toolKeyWithArg]; ok {
				for category, compiledPatterns := range patterns {
					for _, cp := range compiledPatterns {
						if cp.Re.MatchString(line) {
							scores[category] += cp.Weight
						}
					}
				}
			}
		}
		if patterns, ok := pm.compiledToolPatterns[toolKey]; ok {
			for category, compiledPatterns := range patterns {
				for _, cp := range compiledPatterns {
					if cp.Re.MatchString(line) {
						scores[category] += cp.Weight
					}
				}
			}
		}
	}

	// Check against global patterns using precompiled regexes
	for category, compiledPatterns := range pm.compiledOutputPatterns {
		for _, cp := range compiledPatterns {
			if cp.Re.MatchString(line) {
				scores[category] += cp.Weight
			}
		}
	}

	// Strong signal: file:line pattern combined with existing error score
	hasFileLine := pm.fileLinePattern.MatchString(line)
	if hasFileLine {
		// File:line patterns strongly indicate errors (importance 4, not 5)
		scores[TypeError] += 3
		context.Importance = 4 // Slightly lower than explicit error prefixes
	}

	// Strong signal: PASS/FAIL test results
	if pm.passFailPattern.MatchString(line) {
		if strings.HasPrefix(line, "PASS") {
			scores[TypeSuccess] += 5
			context.Importance = 3
		} else {
			scores[TypeError] += 5
			context.Importance = 5
			context.CognitiveLoad = LoadHigh
		}
	}

	// Find category with highest score
	if len(scores) > 0 {
		bestCategory := TypeDetail
		bestScore := 0
		for category, score := range scores {
			if score > bestScore {
				bestScore = score
				bestCategory = category
			}
		}
		// Only adjust importance if not already set by special patterns
		if context.Importance == 2 {
			return adjustCategoryImportance(bestCategory, &context)
		}
		// Apply cognitive load but keep importance from special patterns
		if bestCategory == TypeError && context.CognitiveLoad != LoadHigh {
			context.CognitiveLoad = LoadHigh
		} else if bestCategory == TypeWarning && context.CognitiveLoad == LoadLow {
			context.CognitiveLoad = LoadMedium
		}
		return bestCategory, context
	}

	// Default to detail type with medium importance
	return TypeDetail, context
}

// findToolConfig looks for a configuration for the specific tool being executed.
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
// and returns the category type along with the updated context.
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

// FindSimilarLines groups similar output lines for summarization.
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

// extractPatternKey creates a representative key for a line to group similar outputs.
// Uses a precompiled regex pattern for file:line matching.
func (pm *PatternMatcher) extractPatternKey(content, lineType string) string {
	// Different extraction strategies based on line type
	switch lineType {
	case TypeError, TypeWarning:
		// For errors and warnings, use file:line format as base if present
		// Reuse the fileLinePattern which matches similar format
		if matches := pm.fileLinePattern.FindStringSubmatch(content); len(matches) > 0 {
			return lineType + "_" + matches[0]
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

// DetermineCognitiveLoad analyzes output to determine overall cognitive load.
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

	// Research-based heuristics for cognitive load estimation.
	//
	// Error threshold (5): Based on Miller's Law (1956) - working memory capacity of 7±2 items.
	// Five errors represent high cognitive load, requiring simplified rendering.
	//
	// Output size thresholds:
	//   - High (>100 lines): Requires scrolling, significantly increases cognitive load
	//   - Medium (>30 lines): Approximately one screen of output, moderate cognitive load
	//
	// Warning threshold (3): Lower than errors since warnings are less critical, but
	// multiple warnings indicate potential issues requiring attention.
	//
	// References:
	//   - Sweller, J. (1988). "Cognitive load during problem solving: Effects on learning."
	//   - Miller, G. A. (1956). "The magical number seven, plus or minus two."
	//   - See docs/RESEARCH_FOUNDATIONS.md for detailed citations.
	if errorCount > 5 || outputSize > 100 {
		return LoadHigh
	} else if errorCount > 0 || warningCount > 3 || outputSize > 30 {
		return LoadMedium
	}

	return LoadLow
}
