# vestige-pair — repo review

Run ID: `bd775e303d86-vestige-pair`
Target: `/Users/vcto/Projects/fo` (whole repo, excluding `vendor/`, `_test.go`)
Cap: 10
Findings: 0

## Scope and method

Searched all `.go` files outside `vendor/` and `_test.go` for:

- Canonical empty structs (`type T struct{}`)
- Near-empty structs (≤1 field, zero methods)
- Lone constructors (`func newT() *T { return &T{} }` shape)

Then cross-checked each candidate's type refs and constructor callers across the workspace including tests.

## Candidates examined

### Examined and excluded: `pkg/wrapper/wrapjscpd/jscpd.go:24` (`jscpd struct{}`)

- Decl: `pkg/wrapper/wrapjscpd/jscpd.go:24` — `type jscpd struct{}`
- Constructor: `pkg/wrapper/wrapjscpd/jscpd.go:26` — `func newJscpd() *jscpd { return &jscpd{} }`
- Method on type: `(*jscpd).Convert` at `pkg/wrapper/wrapjscpd/jscpd.go:31` — real implementation, not a no-op.
- Type also instantiated inline at `pkg/wrapper/wrapjscpd/convert.go:9`: `(&jscpd{}).Convert(r, w)` — exported package entry point.
- Constructor `newJscpd` has 5 callers in `pkg/wrapper/wrapjscpd/jscpd_test.go` (lines 15, 42, 79, 97, 105).

Excluded by rule: *"types whose constructor has a test caller (`_test.go`) — test-only types use the `goconst`-style suppression pattern instead."* Additionally, the type is instantiated by the package's exported `Convert` and carries a real method — not scaffolding.

## Result

No empty-or-near-empty struct in the production codebase is paired with a lone, fully-unreferenced constructor. Zero vestiges is the expected outcome for a healthy workspace (per linter prelude).

Tier: action — **no action required**.

---

Caps: searched 100% of in-scope `.go` files; one candidate examined; zero emitted findings.
