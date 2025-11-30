// Package adapter provides stream adapters for parsing structured command output
// into rich visualization patterns.
package adapter

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// MCPInterviewerAdapter parses MCP Interviewer JSON output into QualityReport patterns.
//
// MCP Interviewer produces JSON with tool definitions, scorecards from LLM-based
// quality evaluation, constraint violations, and server information.
type MCPInterviewerAdapter struct{}

// Name returns the adapter name.
func (a *MCPInterviewerAdapter) Name() string {
	return "mcp-interviewer"
}

// Detect checks if the output is MCP Interviewer JSON format.
// Handles both compact and pretty-printed JSON by looking for characteristic
// field names across multiple lines.
func (a *MCPInterviewerAdapter) Detect(firstLines []string) bool {
	if len(firstLines) == 0 {
		return false
	}

	// Join first lines and look for MCP Interviewer markers
	// This handles both compact and pretty-printed JSON
	combined := strings.Join(firstLines, " ")
	
	// MCP Interviewer JSON has these characteristic fields
	hasInitialize := strings.Contains(combined, `"initialize_result"`)
	hasProtocol := strings.Contains(combined, `"protocolVersion"`)
	hasTools := strings.Contains(combined, `"tools"`)
	hasScorecards := strings.Contains(combined, `"tool_scorecards"`)
	hasServerInfo := strings.Contains(combined, `"serverInfo"`)

	// Accept if we see initialize_result + protocolVersion (the core MCP handshake)
	if hasInitialize && hasProtocol {
		return true
	}
	
	// Or if we see tools/scorecards with any MCP marker
	if (hasTools || hasScorecards) && (hasInitialize || hasServerInfo) {
		return true
	}

	return false
}

// mcpInterviewerJSON represents the MCP Interviewer JSON output structure.
type mcpInterviewerJSON struct {
	Parameters       json.RawMessage `json:"parameters"`
	InitializeResult struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	} `json:"initialize_result"`
	Tools          []mcpTool        `json:"tools"`
	Resources      []mcpResource    `json:"resources"`
	ToolScorecards []toolScorecard  `json:"tool_scorecards"`
}

type mcpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type mcpResource struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type toolScorecard struct {
	ToolName struct {
		Length         scoreEntry `json:"length"`
		Uniqueness     scoreEntry `json:"uniqueness"`
		Descriptiveness scoreEntry `json:"descriptiveness"`
	} `json:"tool_name"`
	ToolDescription struct {
		Length     scoreEntry `json:"length"`
		Parameters scoreEntry `json:"parameters"`
		Examples   scoreEntry `json:"examples"`
	} `json:"tool_description"`
	ToolInputSchema struct {
		Complexity  scoreEntry `json:"complexity"`
		Parameters  scoreEntry `json:"parameters"`
		Optionals   scoreEntry `json:"optionals"`
		Constraints scoreEntry `json:"constraints"`
	} `json:"tool_input_schema"`
	ToolOutputSchema struct {
		Complexity  scoreEntry `json:"complexity"`
		Parameters  scoreEntry `json:"parameters"`
		Optionals   scoreEntry `json:"optionals"`
		Constraints scoreEntry `json:"constraints"`
	} `json:"tool_output_schema"`
}

type scoreEntry struct {
	Score         string `json:"score"`
	Justification string `json:"justification"`
}

// Parse converts MCP Interviewer JSON output into a QualityReport pattern.
func (a *MCPInterviewerAdapter) Parse(output io.Reader) (design.Pattern, error) {
	// Read all content
	var buf strings.Builder
	scanner := bufio.NewScanner(output)
	// Increase buffer size for large JSON
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var data mcpInterviewerJSON
	if err := json.Unmarshal([]byte(buf.String()), &data); err != nil {
		return nil, err
	}

	// Build the quality report
	report := &design.QualityReport{
		ServerName:    data.InitializeResult.ServerInfo.Name,
		ServerVersion: data.InitializeResult.ServerInfo.Version,
		Protocol:      data.InitializeResult.ProtocolVersion,
		ToolCount:     len(data.Tools),
		ResourceCount: len(data.Resources),
	}

	// Aggregate scores by category
	report.Categories = []design.QualityCategory{
		{Name: "Name", Passed: 0, Total: 0},
		{Name: "Description", Passed: 0, Total: 0},
		{Name: "Input Schema", Passed: 0, Total: 0},
		{Name: "Output Schema", Passed: 0, Total: 0},
	}

	// Track failures for "needs attention" section
	failureTypes := make(map[string][]string)

	for i, sc := range data.ToolScorecards {
		toolName := ""
		if i < len(data.Tools) {
			toolName = data.Tools[i].Name
		}

		// Tool Name scores
		countScore(&report.Categories[0], sc.ToolName.Length, toolName, "name.length", failureTypes)
		countScore(&report.Categories[0], sc.ToolName.Uniqueness, toolName, "name.uniqueness", failureTypes)
		countScore(&report.Categories[0], sc.ToolName.Descriptiveness, toolName, "name.descriptiveness", failureTypes)

		// Tool Description scores
		countScore(&report.Categories[1], sc.ToolDescription.Length, toolName, "description.length", failureTypes)
		countScore(&report.Categories[1], sc.ToolDescription.Parameters, toolName, "description.parameters", failureTypes)
		countScore(&report.Categories[1], sc.ToolDescription.Examples, toolName, "description.examples", failureTypes)

		// Input Schema scores
		countScore(&report.Categories[2], sc.ToolInputSchema.Complexity, toolName, "input.complexity", failureTypes)
		countScore(&report.Categories[2], sc.ToolInputSchema.Parameters, toolName, "input.parameters", failureTypes)
		countScore(&report.Categories[2], sc.ToolInputSchema.Optionals, toolName, "input.optionals", failureTypes)
		countScore(&report.Categories[2], sc.ToolInputSchema.Constraints, toolName, "input.constraints", failureTypes)

		// Output Schema scores
		countScore(&report.Categories[3], sc.ToolOutputSchema.Complexity, toolName, "output.complexity", failureTypes)
		countScore(&report.Categories[3], sc.ToolOutputSchema.Parameters, toolName, "output.parameters", failureTypes)
		countScore(&report.Categories[3], sc.ToolOutputSchema.Optionals, toolName, "output.optionals", failureTypes)
		countScore(&report.Categories[3], sc.ToolOutputSchema.Constraints, toolName, "output.constraints", failureTypes)
	}

	// Convert failure types to issues list
	for failureType, tools := range failureTypes {
		report.Issues = append(report.Issues, design.QualityIssue{
			Category:   failureType,
			ToolCount:  len(tools),
			ToolNames:  tools,
		})
	}

	// Add constraint violation for tool count
	if len(data.Tools) > 20 {
		report.Constraints = append(report.Constraints, design.ConstraintViolation{
			Code:     "OTC",
			Message:  "Tool count exceeds limit",
			Severity: "warning",
			Details:  len(data.Tools),
			Limit:    20,
		})
	}

	return report, nil
}

func countScore(cat *design.QualityCategory, entry scoreEntry, toolName, failureKey string, failures map[string][]string) {
	if entry.Score == "" {
		return // N/A
	}
	cat.Total++
	if entry.Score == "pass" {
		cat.Passed++
	} else {
		failures[failureKey] = append(failures[failureKey], toolName)
	}
}
