# Hygiene Formats Guide

`fo` recognizes three lightweight, line-oriented formats for "hygiene" output —
the kind of report a Makefile target or doctor script wants to render: a count
distribution, a contract checklist, or a numeric rollup. Each is auto-detected
from a header line on stdin; no flag required.

## What fo accepts on stdin

| Shape                        | Header                | Renderer       | Source                                              |
|------------------------------|-----------------------|----------------|-----------------------------------------------------|
| SARIF 2.1.0                  | `{"version":"2.1.0"…` | findings table | golangci-lint, gosec, custom analyzers              |
| `go test -json` events       | `{"Action":…}`        | test summary   | `go test -json ./...`                               |
| Multiplex (`--- tool: …`)    | first line            | sectioned      | scripts that pipe several tools through one stream  |
| **`# fo:tally`**             | `# fo:tally`          | leaderboard    | `fo wrap leaderboard`, or bare `count<sp>label`     |
| **`# fo:status`**            | `# fo:status`         | PASS/FAIL list | doctor scripts, contract tables                     |
| **`# fo:metrics`**           | `# fo:metrics`        | keyed values   | coverage rollups, bench results, build sizes        |

Bare `<count> <label>` rows (`uniq -c` style) auto-detect as tally when there
are ≥2 such rows and no other content. Anything else falls through to SARIF /
test-json parsing or returns exit code 2.

## `--as <kind>` hint flag

When stdin lacks a header and auto-detection guesses wrong (or you want to be
explicit), pass `--as`:

```bash
mycmd | fo --as tally       # treat input as count<sp>label tally
mycmd | fo --as status      # prepend "# fo:status" before parsing
mycmd | fo --as metrics     # prepend "# fo:metrics"
mycmd | fo --as diag        # parse as line diagnostics → SARIF
```

## Wrappers

Each `fo wrap <name>` reads its tool's native output on stdin and emits one of
fo's accepted formats on stdout, ready to pipe into `fo`.

| Subcommand              | Consumes                              | Emits           |
|-------------------------|---------------------------------------|-----------------|
| `fo wrap archlint`      | go-arch-lint JSON                     | SARIF           |
| `fo wrap archlint-text` | go-arch-lint plain text               | SARIF           |
| `fo wrap cover`         | `go tool cover -func` text            | `# fo:metrics`  |
| `fo wrap diag`          | `file:line:col: msg` lines            | SARIF           |
| `fo wrap gobench`       | raw `go test -bench` text             | `# fo:metrics`  |
| `fo wrap jscpd`         | jscpd JSON duplication report         | SARIF           |
| `fo wrap leaderboard`   | `<count> <label>` rows                | `# fo:tally`    |

## Migration recipes

### `arch-lint` target

```diff
- arch-lint:
- 	@go-arch-lint check ./... | grep '^\[' | awk '{...}'
+ arch-lint:
+ 	@go-arch-lint check ./... | fo wrap archlint-text | fo
```

### `doctor` target

Replace `printf` columns with status rows:

```sh
# scripts/doctor.sh
{
  echo "# fo:status tool=doctor"
  command -v dolt   >/dev/null && echo "ok   dolt-installed" \
                                || echo "fail dolt-installed"$'\t'"not on PATH"
  command -v snipe  >/dev/null && echo "ok   snipe-installed" \
                                || echo "fail snipe-installed"
} | fo
```

### `eval:trend` target

Emit one metrics row per measurement; `fo` shows deltas vs the prior run via
the sidecar history.

```sh
{
  echo "# fo:metrics tool=eval"
  echo "accuracy   $(jq -r .accuracy results.json)   %"
  echo "latency_p95 $(jq -r .p95 results.json)       ms"
} | fo
```

### Build size

```sh
{
  echo "# fo:metrics tool=size"
  for bin in dist/*; do
    echo "$(basename "$bin") $(stat -f%z "$bin") bytes"
  done
} | fo
```

### Bench

```bash
go test -bench=. -count=5 ./... | fo wrap gobench | fo
```

A real benchstat-tabular wrapper (delta columns, geomean rows) is on the
deferred list; until it ships, raw `go test -bench` text is the supported
input.

## State

`fo` keeps a sidecar at `.fo/last-run.json` (findings) and
`.fo/metrics-history.json` (metric samples) for diff classification. Set
`FO_STATE_DIR` to relocate both — useful in CI or test runs that should not
share state with a developer's local sidecar.
