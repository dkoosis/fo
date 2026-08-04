# JSON Shape Review — fo repo

Run: 7858b3a8ea1b
Reviewer: json-shape linter (haiku)

---

### 1. [F1] Unrecognized `omitzero` JSON tag — behaves as no directive

**Diagnosis:** Two struct fields use the JSON tag directive `omitzero`, which is not recognized by Go's encoding/json package. The directive is silently ignored, causing the fields to always marshal (not omit when zero).

**Why:** Standard JSON tag directives are `omitempty`, `-`, and `string`. A typo like `omitzero` — whether intended as custom logic or a misspelling of `omitempty` — will be silently treated as an unknown directive and ignored. The field then always appears in the marshaled JSON.

Intent appears to be omitting fields when they are all-zero (since Beat can be Narration OR Command, not both), but the current code always emits the field.

**Evidence:**

File: `/Users/vcto/Projects/fo/pkg/scene/scene.go` (lines 56–60)
```go
type Beat struct {
	Kind      BeatKind `json:"kind"`
	Narration string   `json:"narration,omitempty"`
	Command   Command  `json:"command,omitzero"`
}
```

File: `/Users/vcto/Projects/fo/pkg/sarif/types.go` (lines 74–77)
```go
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region,omitzero"`
}
```

**Fix:**

Change `omitzero` to `omitempty` to actually omit fields when the struct is the zero value:

BEFORE:
```go
type Beat struct {
	Kind      BeatKind `json:"kind"`
	Narration string   `json:"narration,omitempty"`
	Command   Command  `json:"command,omitzero"`
}
```

AFTER:
```go
type Beat struct {
	Kind      BeatKind `json:"kind"`
	Narration string   `json:"narration,omitempty"`
	Command   Command  `json:"command,omitempty"`
}
```

BEFORE:
```go
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region,omitzero"`
}
```

AFTER:
```go
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region,omitempty"`
}
```

**Tier:** 🔴 High — silent wire-format change. Clients decoding JSON with omitted Command/Region fields will read the zero value, same as if the field were present but zero. But if the intent is to omit these fields and the tag is being ignored, the contract with downstream consumers is broken.

---

### 2. [F2] `omitempty` on int fields loses meaningful zero distinction

**Diagnosis:** Several int fields — line/column coordinates in source locations — use `omitempty`. This causes a field with value 0 to be omitted from the output, making it impossible for a decoder to distinguish "field absent" from "field = 0".

**Why:** While 0 is often an invalid line or column (line numbers are typically 1-based), some tools use 0-based indexing or 0 as a sentinel meaning "not applicable". The `omitempty` tag on an int field collapses two distinct states onto one: `{"line":0}` and `{}` both decode to the same struct.

**Evidence:**

File: `/Users/vcto/Projects/fo/pkg/report/report.go` (lines 34–49)
```go
type Finding struct {
	ID          string   `json:"id,omitempty"`
	RuleID      string   `json:"rule_id,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Col         int      `json:"col,omitempty"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	FixCommand  string   `json:"fix_command,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Score       float64  `json:"score"`
}
```

File: `/Users/vcto/Projects/fo/pkg/sarif/types.go` (lines 84–90)
```go
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}
```

**Fix:**

Replace `omitempty` with a `*int` pointer, so `nil` means absent and 0 means explicit zero:

BEFORE (report.go):
```go
type Finding struct {
	Line        int      `json:"line,omitempty"`
	Col         int      `json:"col,omitempty"`
	// ...
}
```

AFTER (report.go):
```go
type Finding struct {
	Line        *int     `json:"line,omitempty"`
	Col         *int     `json:"col,omitempty"`
	// ...
}
```

BEFORE (sarif/types.go):
```go
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}
```

AFTER (sarif/types.go):
```go
type Region struct {
	StartLine   *int `json:"startLine,omitempty"`
	StartColumn *int `json:"startColumn,omitempty"`
	EndLine     *int `json:"endLine,omitempty"`
	EndColumn   *int `json:"endColumn,omitempty"`
}
```

**Tier:** 🟡 Medium — domain expressivity loss. Unlikely to affect typical 1-based linter output (since 0 is invalid), but violates the principle that JSON fields express distinct domain states. Also requires auditing all code that reads these fields to handle `nil`.

---

### 3. [F3] SARIF decoder does not reject unknown fields at trust boundary

**Diagnosis:** The SARIF parser at `/Users/vcto/Projects/fo/pkg/sarif/reader.go` uses `json.NewDecoder` without calling `.DisallowUnknownFields()`. This means unknown fields in incoming JSON are silently dropped rather than rejected.

**Why:** The SARIF decoder is called from `cmd/fo/parse.go` (the main CLI parser) to process external tool output from stdin — a trust boundary. A typo in a field name (e.g., `message` instead of `msg`) will silently be ignored rather than surfaced as an error, potentially causing downstream logic to operate on zero values instead of intended data.

While the code includes a comment about tolerating trailing data (for golangci-lint v2 compatibility), that is distinct from unknown fields within the JSON structure. Trailing data is handled by the decoder's inherent tolerance; unknown fields within the structure should be rejected.

**Evidence:**

File: `/Users/vcto/Projects/fo/pkg/sarif/reader.go` (lines 27–41)
```go
// Read parses SARIF from an io.Reader.
func Read(r io.Reader) (*Document, error) {
	dec := json.NewDecoder(r)
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode sarif: %w", err)
	}
	// Trailing data is tolerated: golangci-lint v2 appends a text summary
	// after the SARIF JSON document, and the decoder already consumed the
	// complete first JSON value successfully.
	if doc.Version == "" {
		return nil, errMissingSARIFVersion
	}
	return &doc, nil
}
```

Callsite in `cmd/fo/parse.go` (line 154):
```go
doc, err := sarif.ReadBytes(input)
```

This is invoked during main CLI parsing of external (untrusted) tool output.

**Fix:**

Add `.DisallowUnknownFields()` before decoding:

BEFORE:
```go
func Read(r io.Reader) (*Document, error) {
	dec := json.NewDecoder(r)
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode sarif: %w", err)
	}
	// ...
}
```

AFTER:
```go
func Read(r io.Reader) (*Document, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode sarif: %w", err)
	}
	// ...
}
```

Also apply to `checkDepth` (line 60) which uses a separate decoder for depth checking:
```go
dec := json.NewDecoder(bytes.NewReader(data))
dec.DisallowUnknownFields()
```

(Note: `checkDepth` only calls `.Token()`, not `.Decode()`, so unknown fields won't cause an error there; however, for consistency and defense-in-depth, add the call.)

**Tier:** 🟡 Medium — input validation at trust boundary. Unlikely to cause immediate crashes, but violates the principle of validating external input. A tool bug (typo'd field) could silently corrupt the analysis by dropping fields.

---

Summary: 3 findings. All are JSON shape issues where the wire format does not faithfully express domain semantics or external input is not validated.
