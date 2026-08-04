# sim-pair — repo review

Run ID: `7858b3a8ea1b`
Target: `/Users/vcto/Projects/fo` (whole repo, one condensed pass across all packages)
Cap: 10
Findings: 0

## Scope and method

`sim-pair` flags semantic near-duplicate functions — same package, `snipe sim`
cosine similarity ≥0.9, callee-set overlap ≤50% — using embedding search over
symbol bodies. The linter's own pre-work step gates on embeddings being ready
before it does anything else:

```
if ! snipe embed-status 2>/dev/null | grep -q 'embeddings: ready'; then
  echo "snipe embeddings unavailable — run 'snipe index --embed' first; sim-pair is a no-op without them"
  exit 0
fi
```

## Result: no-op — embeddings unavailable

Ran the gate check against this workspace:

```
$ snipe embed-status
Batch batch-7vsqB8z1RhPTFyXfAM7vxHEuMDnj2SLv8BFkt: status=failed, completed=0/789, failed=0
{"...,"status":"failed","total":789,"completed":0,"failed":0,"model":"voyage-code-3",...}
```

No batch has ever completed (`completed=0/789`); the last embed job is marked
`status=failed`. Confirmed directly against `snipe sim`, which is the primitive
every candidate pair in this linter depends on:

```
$ snipe sim "func ParseBytes(data []byte) (*report.Report, error)" --threshold 0.9 --limit 5 --format json
{"ok":false,"results":[],"error":{"code":"INTERNAL_ERROR","message":"no embeddings found. Run 'snipe index --embed' first", ...}}
```

Per the linter's own degrade-gracefully instruction, this is a clean no-op, not
a "zero pairs found" result — there is no similarity signal to search over.
Producing findings by falling back to a different method (e.g. eyeballing
package inventories for name/shape similarity) would not satisfy the rule's
evidence requirement (similarity score + callee-overlap % from `snipe sim`),
so none are reported.

To make this linter functional, re-run `snipe index --embed` (789 symbols)
outside of a report-only pass — this run does not modify the working tree or
kick off that batch job, per the reviewer's report-mode contract.

Tier: action — **infra gap, not a code finding**: get embeddings indexed before
the next `sim-pair` pass, otherwise every future run degrades to this same
no-op.

---

Caps: pre-work gate evaluated per linter spec; 0 candidate pairs examined
(blocked upstream of the search); 0 emitted findings.
