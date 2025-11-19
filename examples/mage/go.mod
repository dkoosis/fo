module github.com/dkoosis/fo/examples/mage

go 1.24.2

require github.com/dkoosis/fo v0.0.0

require (
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.32.0 // indirect
	golang.org/x/text v0.25.0 // indirect
)

// NOTE: This replace directive is for local development only.
// For standalone usage, remove this line and ensure github.com/dkoosis/fo
// is available via go modules. For local development, you can use:
//   go work init ../../ examples/mage
//   go work use .
// instead of modifying go.mod
replace github.com/dkoosis/fo => ../..
