package main

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/pkg/adapter"
	"github.com/dkoosis/fo/pkg/design"
)

func main() {
	// Example Go test JSON output
	goTestJSON := `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/api"}
{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/api","Test":"TestGetUser"}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/api","Test":"TestGetUser","Elapsed":0.15}
{"Time":"2024-01-01T12:00:01Z","Action":"run","Package":"pkg/api","Test":"TestCreateUser"}
{"Time":"2024-01-01T12:00:02Z","Action":"pass","Package":"pkg/api","Test":"TestCreateUser","Elapsed":0.23}
{"Time":"2024-01-01T12:00:02Z","Action":"pass","Package":"pkg/api","Elapsed":0.38}
{"Time":"2024-01-01T12:00:02Z","Action":"run","Package":"pkg/database"}
{"Time":"2024-01-01T12:00:02Z","Action":"run","Package":"pkg/database","Test":"TestConnect"}
{"Time":"2024-01-01T12:00:03Z","Action":"fail","Package":"pkg/database","Test":"TestConnect","Elapsed":0.05}
{"Time":"2024-01-01T12:00:03Z","Action":"fail","Package":"pkg/database","Elapsed":0.05}
{"Time":"2024-01-01T12:00:03Z","Action":"run","Package":"pkg/utils"}
{"Time":"2024-01-01T12:00:03Z","Action":"run","Package":"pkg/utils","Test":"TestFormat"}
{"Time":"2024-01-01T12:00:03Z","Action":"pass","Package":"pkg/utils","Test":"TestFormat","Elapsed":0.01}
{"Time":"2024-01-01T12:00:03Z","Action":"run","Package":"pkg/utils","Test":"TestValidate"}
{"Time":"2024-01-01T12:00:04Z","Action":"skip","Package":"pkg/utils","Test":"TestValidate","Elapsed":0.0}
{"Time":"2024-01-01T12:00:04Z","Action":"pass","Package":"pkg/utils","Elapsed":0.01}`

	fmt.Println("=== Stream Adapter Demo ===\n")
	fmt.Println("Raw Go Test JSON Output:")
	fmt.Println(strings.Repeat("-", 60))
	// Show first few lines of raw JSON
	lines := strings.Split(goTestJSON, "\n")
	for i := 0; i < 3 && i < len(lines); i++ {
		fmt.Println(lines[i])
	}
	fmt.Println("... (" + fmt.Sprintf("%d", len(lines)) + " total lines)")
	fmt.Println()

	// Create adapter registry
	registry := adapter.NewRegistry()

	// Detect format from first few lines
	firstLines := lines[:5]
	streamAdapter := registry.Detect(firstLines)

	if streamAdapter == nil {
		fmt.Println("No adapter detected for this format")
		return
	}

	fmt.Printf("Detected format: %s\n\n", streamAdapter.Name())

	// Parse into pattern
	reader := strings.NewReader(goTestJSON)
	pattern, err := streamAdapter.Parse(reader)
	if err != nil {
		fmt.Printf("Error parsing: %v\n", err)
		return
	}

	// Render with theme
	cfg := design.UnicodeVibrantTheme()
	fmt.Println("Parsed and Rendered Output:")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println(pattern.Render(cfg))

	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("\nBenefits:")
	fmt.Println("  • Automatic format detection")
	fmt.Println("  • Structured visualization (TestTable)")
	fmt.Println("  • Status icons and colors")
	fmt.Println("  • Failure details highlighted")
	fmt.Println("  • Much easier to scan than raw JSON")
}
