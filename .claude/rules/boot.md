# Boot
updated: 2026-05-18

→ next: `bd ready` — /clean pass landed; 9 audit findings remain (5 cosmetic test goconst + 1 arch report-only + 3 production rangeValCopy already fixed).

✓ done
- fo-u15.3.3 shipped 9 TDD commits: cluster render (collapsed human + Shape A/B llm + --expand flag), pushed `a738b58`
- /clean pass: 19+ audit findings resolved across gofmt/goconst/err113/gosec/nestif/gocognit/rangeValCopy/forcetypeassert tiers, pushed `d5ef42a`

‡ traps
- `bd dep add <child> <epic>` errors ("tasks can only block tasks") — use `bd update <child> --parent <epic>`
- LSP diagnostics sometimes lag behind file edits (stale `it.Cluster undefined` warnings) — verify with `go build ./...` not the IDE stream
