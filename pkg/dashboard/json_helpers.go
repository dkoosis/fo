package dashboard

import (
	"encoding/json"
	"strings"
)

func decodeJSONLines(lines []string, out any) bool {
	fullOutput := strings.Join(lines, "\n")
	return decodeJSONPayload(fullOutput, out)
}

func decodeJSONLinesWithPrefix(lines []string, out any) bool {
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return false
	}
	return decodeJSONPayload(fullOutput[jsonStart:], out)
}

func decodeJSONPayload(payload string, out any) bool {
	if err := json.Unmarshal([]byte(payload), out); err != nil {
		return false
	}
	return true
}
