package version

// These variables are populated by the Go linker (LDFLAGS) at build time.
var (
	Version    = "dev"     // Default value if not built with LDFLAGS
	CommitHash = "unknown" // Default value
	BuildDate  = "unknown" // Default value
)
