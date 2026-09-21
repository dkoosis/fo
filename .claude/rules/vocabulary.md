# fo vocabulary

The repo's own words. Use these; don't coin synonyms.

| Word | Means |
|---|---|
| Report | The IR: one snapshot of findings, tests, diff, notices. Every parser produces it, every renderer consumes it. |
| Scene | The second IR, for narrated multi-actor walkthroughs over time (`pkg/scene`). A bounded exception, not a precedent. |
| finding | One diagnostic from a linter or scanner (SARIF result). |
| handle | Short stable id for a finding or test (`F-7a2`, `T-3f1`): shortest unique fingerprint prefix. `fo explain` resolves it. |
| fingerprint | Finding identity across runs; drives diff classification. |
| diff class | new / persistent / fixed, from the sidecar `.fo/last-run.json`. |
| sidecar | State fo keeps under `.fo/`: `last-run.json`, `findings.json`, `run-log.json`. |
| hygiene format | A shape declaration on stdin: `# fo:status`, `# fo:metrics`, `# fo:tally`. |
| shape | The data's structure (counts, ratios, series, key/value, pass/fail rows); it picks the visual idiom. |
| wrapper | A thin `pkg/wrapper/*` adapter that converts a foreign format to SARIF or a hygiene format, then exits. |
| multiplex | The `--- tool: ---` protocol that carries several tools' output in one stream. |
| human / llm / json | The three renderers. Auto: TTY is human, piped is llm. Peers, none is the "real" one. |
