# Candidate Report — Go consolidation/simplification

## Baseline QA status
- `mage qa`: not available in this environment (`mage` not found).
- `go test ./...`: ✅ completed in ~1m17s.
- `gocyclo` / `scc` / `cloc`: not installed.

## Discovery map (high-level)
- Dashboard formatters in `pkg/dashboard/*` contain repeated JSON extraction/parsing and fallback logic.
- SARIF reader in `pkg/sarif/reader.go` duplicates decode/validation paths.
- Multiple formatters do nearly identical “find JSON in mixed output” logic.
- JSON report parsing appears across `pkg/dashboard/formatter_*.go` with similar control flow.

## Ranked consolidation candidates (10+)
Ranking heuristic: (leverage * safety * clarity) / (risk * churn)

1. **Unify JSON parsing in dashboard formatters**
   - Locations: `pkg/dashboard/formatter_kgbaseline.go`, `formatter_orcahygiene.go`, `formatter_nugstats.go`, `formatter_mcperrors.go`, `formatter_telemetrysignals.go`.
   - Duplication: repeated `strings.Join` + `strings.Index("{")` + `json.Unmarshal` + fallback.
   - Proposal: shared helper for “JSON with leading noise” decoding.
   - Expected win: ~30–40 LOC removed; fewer parsing variants.
   - Risk: Low.
   - Tests: reuse existing formatter tests.
   - Migration: replace inline blocks with helper.

2. **Unify JSON parsing for clean dashboard outputs**
   - Locations: `pkg/dashboard/formatter_archlint.go`, `formatter_filesize.go`, `formatter_sarif.go`.
   - Duplication: `strings.Join` + `json.Unmarshal` + fallback.
   - Proposal: shared helper for straight JSON decoding.
   - Expected win: small LOC reduction; consistent behavior.
   - Risk: Low.
   - Tests: reuse existing tests.
   - Migration: swap parsing blocks.

3. **Consolidate SARIF reader validation**
   - Location: `pkg/sarif/reader.go`.
   - Duplication: `Read` and `ReadBytes` each decode + validate version.
   - Proposal: single validation helper; have `ReadBytes` use `Read`.
   - Expected win: smaller reader surface; consistent error path.
   - Risk: Low.
   - Tests: existing SARIF tests.
   - Migration: extract `validateDocument`.

4. **Standardize JSON probe in dashboard status methods**
   - Locations: `formatter_kgbaseline.go`, `formatter_orcahygiene.go`, `formatter_nugstats.go`, `formatter_mcperrors.go`.
   - Duplication: status methods re-implement JSON extraction.
   - Proposal: reuse helper for status path.
   - Expected win: less drift between format and status logic.
   - Risk: Low.
   - Tests: existing status indicator tests.

5. **Normalize file read/parse helpers in SARIF package**
   - Locations: `pkg/sarif/reader.go`, `pkg/sarif/renderer.go`, `pkg/sarif/orchestrator.go`.
   - Duplication: multiple file read + parse pathways.
   - Proposal: consolidate file reading to `ReadFile` where applicable.
   - Expected win: smaller API surface.
   - Risk: Low/Medium (call site behavior).
   - Tests: `pkg/sarif` tests.

6. **Shared go test JSON parsing**
   - Locations: `fo/testjson.go`, `pkg/dashboard/formatter_gotest.go`.
   - Duplication: line-level JSON decoding + action handling.
   - Proposal: shared parser or minimal helper to parse NDJSON lines.
   - Expected win: medium LOC reduction.
   - Risk: Medium (different output expectations).
   - Tests: go test JSON parsing tests.

7. **Consolidate “plain fallback” render path**
   - Locations: most `pkg/dashboard/formatter_*.go` files.
   - Duplication: repeated `return (&PlainFormatter{}).Format(...)`.
   - Proposal: helper `plainFallback(lines, width)`.
   - Expected win: small LOC reduction.
   - Risk: Low.
   - Tests: existing.

8. **Unify JSON extraction for “mixed output” formatters**
   - Locations: `formatter_golangcilint.go`, `formatter_govulncheck.go`, others with line scanning.
   - Duplication: parse JSON line-by-line with simple detection.
   - Proposal: helper to scan for JSON line containing a key.
   - Expected win: small, clarity gain.
   - Risk: Medium (heuristics).
   - Tests: existing.

9. **Normalize file/path utilities**
   - Locations: `pkg/dashboard/*` uses `shortPath`, `trimPath`, etc.
   - Duplication: repeated path trimming logic.
   - Proposal: shared helper in dashboard package.
   - Expected win: minor LOC reduction.
   - Risk: Low.

10. **Unify config file load logic**
    - Locations: `internal/config/config.go` read paths repeated.
    - Duplication: repeated `os.ReadFile` + yaml unmarshal + error wrapping.
    - Proposal: helper to load YAML with path + defaults.
    - Expected win: medium LOC reduction.
    - Risk: Medium (config behavior).

11. **Consolidate SARIF rendering counts**
    - Locations: `pkg/sarif/renderer.go`, `pkg/sarif/orchestrator.go`.
    - Duplication: count issues, filter levels.
    - Proposal: move to `ComputeStats` results.
    - Expected win: small code reduction + single source of truth.
    - Risk: Low/Medium.

## Chosen items (implemented)
1. **Dashboard JSON parsing helpers**
   - Before: every formatter repeated `strings.Join`/`Index`/`json.Unmarshal` and fallback.
   - After: centralized helpers in `pkg/dashboard/json_helpers.go` used by multiple formatters.
   - Risk: Low (pure refactor, same parsing behavior).

2. **Dashboard status parsing reuse**
   - Before: status methods repeated JSON extraction logic.
   - After: status methods use shared helper for consistent decode path.
   - Risk: Low.

3. **SARIF reader validation consolidation**
   - Before: `Read` and `ReadBytes` duplicated decode + validation.
   - After: `ReadBytes` reuses `Read`, validation centralized.
   - Risk: Low.

## Deferred notes
- Go test JSON consolidation across `fo/` and `pkg/dashboard/` deferred due to higher behavior risk (different output expectations).
- Config loader consolidation deferred to avoid changing error wrapping and config resolution behavior.

## Environment/tooling notes
- `mage` is not installed, so QACommand could not be run here.
- `gocyclo`, `scc`, and `cloc` are not available; consider adding them to the dev container/tooling.
