# zero-sentinel Review — fo

**RUN_ID:** 7858b3a8ea1b  
**Scope:** project  
**Mode:** report  
**Finding cap:** 10  
**Findings:** 1

---

## 1. [F1] `pkg/report/report.go:42–43` — optional-value-without-pointer

**Diagnosis:**  
The `Finding` struct uses plain `int` fields (`Line` and `Col`) to represent optional source-code location coordinates, treating `0` as "not set" via defensive > 0 checks at read time. Since valid line and column numbers start at 1 in source files, 0 is a safe sentinel in practice — but the type does not express the optionality, and callers must remember the convention.

**Why:**  
When a Finding lacks location data, there is no way for a reader of the struct definition to know that `Line == 0` means "absent" rather than "first line of file". The zero value is conflated with a domain-relevant state (no location available). Changing the struct requires callers to know and maintain the 0→absent convention; a refactored call site can easily forget the guard.

**Evidence:**  
Lines 42–43 in `/Users/vcto/Projects/fo/pkg/report/report.go`:
```go
Line        int      `json:"line,omitempty"`
Col         int      `json:"col,omitempty"`
```

Usage pattern at `/Users/vcto/Projects/fo/pkg/view/github.go:63–68`:
```go
if f.Line > 0 {
	fmt.Fprintf(&b, ",line=%d", f.Line)
}
if f.Col > 0 {
	fmt.Fprintf(&b, ",col=%d", f.Col)
}
```

Additional usage at `/Users/vcto/Projects/fo/cmd/fo/explain.go:67–68`:
```go
if f.Col > 0 {
	loc += fmt.Sprintf(":%d", f.Col)
}
```

And at `/Users/vcto/Projects/fo/cmd/fo/state.go:213–214`:
```go
if f.Line > 0 {
	loc = fmt.Sprintf("%s:%d", f.File, f.Line)
}
```

**Fix:**  
Replace `Line` and `Col` with optional types to make the distinction explicit. Pointer types are the simplest:

```before
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

```after
type Finding struct {
	ID          string   `json:"id,omitempty"`
	RuleID      string   `json:"rule_id,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        *int     `json:"line,omitempty"`
	Col         *int     `json:"col,omitempty"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	FixCommand  string   `json:"fix_command,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Score       float64  `json:"score"`
}
```

Update call sites from `if f.Line > 0` to `if f.Line != nil`. JSON contract remains unchanged — nil/zero omit via `omitempty`. Test the roundtrip to confirm pointer marshaling works as expected.

**Tier:** action  
Findings without location data today are correct, but the conflation poses a subtle latent bug on refactor. Explicit types eliminate the convention burden.
