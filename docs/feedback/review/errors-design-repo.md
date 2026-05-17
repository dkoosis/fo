# errors-design — repo

run: `bd775e303d86-errors-design` · target: repo · mode: report

Semantic review of error shape, wrap quality, handle-once, recover. Mechanical idiom checks (errname/wrapcheck) deferred to those linters.

## Summary

- 3 exported sentinels with **zero production consumers** (only same-package tests `errors.Is`). API surface honesty problem.
- 2 wrappers tag their package name in the inner error while the caller in `cmd/fo/main.go` tags it again — double-prefix in user-visible output.
- No `recover()`, no `log+return`, no typed-nil-as-interface, no HTTP boundary leaks. Those phases come up clean.

Total findings: **5** (cap 10).

---

### 1. [F1] `pkg/sarif/reader.go:12` — sentinel-without-callers

**Diagnosis.** `ErrMissingSARIFVersion` is exported but has **zero callers anywhere**, including tests. The only reference is the producer at line 25.

**Why.** Sentinel exports are an API commitment: callers may `errors.Is` on them. Exporting one nobody uses both pretends a contract exists and prevents you from changing the message later without a fake breaking change. `sarif.Read` / `sarif.ReadBytes` callers in `cmd/fo/main.go:609,826` and the three `sarif/toreport_test.go` sites never branch on this value — they treat the error as opaque.

**Evidence (read-verified).** `rg -n 'sarif\.ErrMissing|ErrMissingSARIFVersion'` returns only the declaration site and the single producer in the same file. `pkg/sarif/reader.go:11-26` confirmed via Read.

**Fix.** Unexport to `errMissingSARIFVersion` (still satisfies err113), or fold into `fmt.Errorf("decode sarif: missing version")` with no sentinel at all. If a future caller actually needs to branch on missing-version, re-export with intent.

**Tier:** P1 / 🟡

---

### 2. [F2] `pkg/tally/tally.go:60,64,69` — sentinel-without-callers

**Diagnosis.** `tally.ErrNoHeader`, `tally.ErrNoRows`, `tally.ErrMalformedRow` are exported sentinels with **zero out-of-package consumers**. Production callers don't import `tally.Err*`.

**Why.** Same shape as F1, but a cluster. The package comment on `ErrMalformedRow` actually says the sentinel exists "to keep err113 happy and let callers `errors.Is` on a single root" — but no external caller does. The `tally` package consumers (rendering in `pkg/view/*`, dispatch in `cmd/fo/main.go`) treat parse failures as opaque and just propagate.

**Evidence (read-verified).** `rg -n 'tally\.Err' --type go` returns zero hits. Same-package tests `pkg/tally/tally_test.go:90,98,106,114` use them but that's not a public contract — internal tests can use unexported names.

**Fix.** Unexport (`errNoHeader`, `errNoRows`, `errMalformedRow`); tests in the same package keep working. If a `fo`-external consumer (e.g., a future plugin) needs to branch, re-export then.

**Tier:** P1 / 🔴 (≥3 sentinels with no callers in one package)

---

### 3. [F3] `pkg/wrapper/wrapleaderboard/wrapleaderboard.go:37,40` — sentinel-without-callers

**Diagnosis.** `wrapleaderboard.ErrNoRows` and `wrapleaderboard.ErrMalformedRow` exported; **only same-package tests** use them. The three production call sites in `cmd/fo/main.go:308,454,1287` invoke `wrapleaderboard.Convert(...)` and treat the error as opaque (print + exit).

**Why.** Same drift as F2. The wrapper packages all share this pattern — a private-by-intent error is exported because the lint tool flags "use a sentinel," but no caller ever branches on it. The lint advice was misapplied: err113's intent is "don't allocate the same error string repeatedly," not "every distinct error must be exported."

**Evidence (read-verified).** `rg -n 'wrapleaderboard\.Err' --type go` shows declarations + tests only; `cmd/fo/main.go:308,454,1287` confirmed not to use `errors.Is`.

**Fix.** Unexport both. The lint rule is about identity (no repeat allocation), not visibility.

**Tier:** P1 / 🟡

---

### 4. [F4] `pkg/wrapper/wraparchlint/archlint.go:32` — wrap-redundant

**Diagnosis.** `Convert` returns `fmt.Errorf("archlint: %w", err)` on the JSON-unmarshal failure. The sole caller in `cmd/fo/main.go:1184` then prints `fmt.Fprintf(stderr, "fo wrap archlint: %v\n", err)`. End-user sees `fo wrap archlint: archlint: invalid character ...` — the second "archlint:" is noise.

**Why.** Cmd-layer already owns the subcommand name as the user-visible context. The library prefix duplicates it. The reading-input wrap at `archlint.go:19` (`"reading input: %w"`) is fine — adds genuinely new info about *which* step failed.

**Evidence (read-verified).** `archlint.go:30-33` + `cmd/fo/main.go:1183-1186` both confirmed via Read.

**Fix.** Drop the prefix: `return fmt.Errorf("unmarshal: %w", err)` (which stage), or just `return err`. Same pattern, same fix in `pkg/wrapper/wrapjscpd/jscpd.go:81` (`fmt.Errorf("jscpd: %w", err)`) — caller already prefixes "fo wrap jscpd:".

**Tier:** P2 / 🟡 (2 sites)

---

### 5. [F5] `pkg/scene/scene.go:224,231,240,265,269,279,284,294,299,303` — wrap-redundant (mild)

**Diagnosis.** Errors like

    return fmt.Errorf("scene: line %d: %w: narration before any act", p.lineNo, errMalformedAct)

triple-tag: outer `"scene:"` + inner `errMalformedAct` whose message is `"scene: malformed act header"` (line 112). Output: `scene: line 5: scene: malformed act header: narration before any act` — "scene:" appears twice.

**Why.** The package-name prefix on the `errMalformed*` sentinels (`scene.go:112-114`) collides with the package-name prefix on the wrap. One should drop it. Tier 3 because it's only mild noise — the line number is the load-bearing context.

**Evidence (read-verified).** `pkg/scene/scene.go:112-114, 224, 231, 240, 265, 269, 279, 284, 294, 299, 303` — every wrap of `errMalformed*` produces a doubled "scene:".

**Fix.** Drop the `"scene: "` prefix from the three sentinel messages:

    errMalformedAct   = errors.New("malformed act header")
    errMalformedActor = errors.New("malformed actor line")
    errMalformedExit  = errors.New("malformed exit trailer")

Outer wrap retains the package tag and line number. Cleaner output, no signal lost.

**Tier:** P2 / 🟢

---

## Phases not findings-bearing

- **handle-once (P1).** No `log.X(err); return err` sites — fo has no logger; errors propagate to `cmd/fo/main.go` which prints and exits. ✓
- **typed-nil-as-error / interface-and-nil (P2).** No `*MyErr` return-variable patterns. The one typed error (`UnknownFormatError`, `pkg/report/multiplex.go:50`) is returned as a value through normal error chains; callers use `errors.As` correctly (`cmd/fo/main.go:751`, `pkg/report/multiplex_test.go:181,202`). Fields are actually read. ✓
- **boundary-leak-to-client (P2).** fo is a CLI; no HTTP/RPC handlers. n/a.
- **silent-recover / recover-without-stack (P3).** Zero `recover()` calls in the repo. ✓
- **typed-error-fields-unread (P1).** `UnknownFormatError` fields `SectionIndex`, `Tool`, `Format` are read by tests (`multiplex_test.go:184-188, 205-207`). Caller in `cmd/fo/main.go:751` uses `errors.As`. Borderline — tests use fields, production reads `Error()` only — but the typed shape is justified by the test-level contract. ✓
