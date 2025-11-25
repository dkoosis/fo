package fo

import "testing"

func TestHumanizeTestName_HandlesStandardFormat_When_ThreePartName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "TestPipelineEndToEnd_RunsPipeline_When_FullPipelineExecuted",
			expected: "Pipeline End To End: Runs Pipeline - When Full Pipeline Executed",
		},
		{
			input:    "TestBackupCreate_ReturnsError_When_ScriptMissing",
			expected: "Backup Create: Returns Error - When Script Missing",
		},
		{
			input:    "TestNewFallbackChain_UsesDefaultStrategies_When_NoStrategiesProvided",
			expected: "New Fallback Chain: Uses Default Strategies - When No Strategies Provided",
		},
		{
			input:    "TestFallbackChainNavigate_StopsOnFirstSuccess_When_StrategySucceeds",
			expected: "Fallback Chain Navigate: Stops On First Success - When Strategy Succeeds",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := HumanizeTestName(tc.input)
			if result != tc.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s", tc.input, tc.expected, result)
			}
		})
	}
}

func TestHumanizeTestName_HandlesAcronyms_When_CommonAcronymsPresent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "TestHTTPClient_SendsRequest_When_APIAvailable",
			expected: "HTTP Client: Sends Request - When API Available",
		},
		{
			input:    "TestMCPServer_HandlesJSON_When_ValidPayload",
			expected: "MCP Server: Handles JSON - When Valid Payload",
		},
		{
			input:    "TestSQLDatabase_ExecutesQuery_When_DBConnected",
			expected: "SQL Database: Executes Query - When DB Connected",
		},
		{
			input:    "TestKGSearchCommand_ReturnsResults_When_ValidQuery",
			expected: "KG Search Command: Returns Results - When Valid Query",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := HumanizeTestName(tc.input)
			if result != tc.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s", tc.input, tc.expected, result)
			}
		})
	}
}

func TestHumanizeTestName_HandlesSimpleNames_When_NoUnderscores(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "TestGet",
			expected: "Get",
		},
		{
			input:    "TestDefaultValues",
			expected: "Default Values",
		},
		{
			input:    "TestString",
			expected: "String",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := HumanizeTestName(tc.input)
			if result != tc.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s", tc.input, tc.expected, result)
			}
		})
	}
}

func TestHumanizeTestName_HandlesTwoPartNames_When_NoCondition(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "TestManager_ConcurrentStateWrites",
			expected: "Manager - Concurrent State Writes",
		},
		{
			input:    "TestMagefileIntegration_Build",
			expected: "Magefile Integration - Build",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := HumanizeTestName(tc.input)
			if result != tc.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s", tc.input, tc.expected, result)
			}
		})
	}
}
