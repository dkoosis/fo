/*
Package adapter turns tool-specific output into structured events and design patterns for fo's pattern-based renderer.

The package defines two extension points: line-oriented [Parser] implementations that classify raw command output into generic
[Event] values, and stream-level [StreamAdapter]s that auto-detect structured formats (like Go test JSON) and convert them into
rich design patterns such as [design.TestTable]. This keeps command execution decoupled from visualization so adapters can
focus on extracting semantics while downstream renderers apply themes and pattern composition described in the project
overview.

Adapters support fo's architecture of capturing command output, recognizing semantic patterns, and rendering dense terminal
dashboards. Registry-based detection lets the CLI and examples fallback to passthrough parsing when no structured format is
recognized while still enabling richer presentations when adapters match formats called out in the stream adapter design and
examples documentation.

# Usage

	registry := adapter.NewRegistry()
	adapter := registry.Detect(firstLines)
	if adapter != nil {
	        pattern, err := adapter.Parse(output)
	        if err != nil {
	                return err
	        }
	        fmt.Println(pattern.Render(design.UnicodeVibrantTheme()))
	}

# API Safety

The zero value of [Registry] contains no adapters; use [NewRegistry] to register the built-in Go test adapter or call
[Registry.Register] to add your own before invoking [Registry.Detect].
*/
package adapter
