package wrapdiag

import "io"

// DiagOpts carries the wrapdiag flags as plain values for the v2 CLI
// dispatch — bypasses the *flag.FlagSet ceremony of the plugin path.
// Tool is required; Rule, Level, Version match the plugin defaults
// when zero ("finding", "warning", "").
type DiagOpts struct {
	Tool    string
	Rule    string
	Level   string
	Version string
}

// Convert reads line diagnostics from r and writes SARIF to w using opts.
// Same semantics as the plugin Convert, but no FlagSet plumbing.
func Convert(r io.Reader, w io.Writer, opts DiagOpts) error {
	if opts.Rule == "" {
		opts.Rule = "finding"
	}
	if opts.Level == "" {
		opts.Level = "warning"
	}
	d := &diag{
		toolName: &opts.Tool,
		ruleID:   &opts.Rule,
		level:    &opts.Level,
		version:  &opts.Version,
	}
	return d.Convert(r, w)
}
