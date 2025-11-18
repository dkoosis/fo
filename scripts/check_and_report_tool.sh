#!/bin/bash

TOOL_CMD="$1"     # Command to check (e.g., golangci-lint)
TOOL_NAME="$2"    # Display name for the tool (e.g., golangci-lint)
# Third argument can be an icon, but fo print will handle icons based on type

# This script's output will be captured by fo if --show-output is used.
# The script itself echoes the detailed status message.
# fo's own inline progress will apply to the label you give it when calling this script.

path_to_tool=$(which "$TOOL_CMD" 2>/dev/null)
if [ -n "$path_to_tool" ]; then
    # Output for fo to capture and display
    echo "$TOOL_NAME already installed: $path_to_tool"
    exit 0
else
    echo "$TOOL_NAME not found. Please install it."
    exit 1 # Indicate failure
fi