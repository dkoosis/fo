# Boot
updated: 2026-06-14

→ next: cast-rail follow-up from docs/design/three-output-rails.md §"Open Requirements Questions" (full-redraw vs delta, playback target, distribution — none block). Or take a ready P1 bug: fo-j4m (lossy omitempty on IR fields), fo-nrx (suppress roundtrip + don't abort on first error).

state: locked: Frame{} in pkg/scene · scene-vs-Report SETTLED (Report singular for snapshots; Scene = bounded exception).

✓ done
- shipped `--format cast`: fo:scene → asciinema v2 recording (afc8ce6). cast.go + asciinema.go + golden whoami_cast.txtar; double-close race fix in stream_cancel_test (sync.Once).
- north-star: codified Scene as bounded exception to Report-is-the-IR.
- merged fo-76n (d4feb0f): untrack generated build/golangci.sarif, goconst test-fixture exemption, real `build` make target.
- removed gas-city integration (35bd239): deleted .gc/ .quality/ identity.toml routes.jsonl; reverted gc.* config + dolt server-mode injection. we don't use it.
- tree clean: 1 branch (main), 1 worktree, 0 stashes.

‡ traps
- bd create/close broken: migration 0047 wisps table missing. reads ok.
- doc-governance pre-commit hook blocks root *.md except README/decisions.log. CHANGELOG.md is UNCOMMITTED+blocked — needs a home decision (relocate to docs/, or --no-verify).
- main ahead of origin, unpushed (push not yet authorized).
