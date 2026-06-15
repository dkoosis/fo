# Changelog

All notable changes to fo are recorded here. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); fo uses semantic-ish tags.

## v0.2.3

### Breaking

- **`fo wrap sarif` renamed to `fo wrap diag`.** The line-diagnostics wrapper
  (`file:line:col: msg` → SARIF 2.1.0) is now `fo wrap diag`. Update any pipeline
  pinned to an older fo, e.g. `go vet ./... 2>&1 | fo wrap diag --tool govet`.
  No alias is kept — old pins calling `wrap sarif` error with
  `unknown subcommand`, and old fo calling `wrap diag` errors the same way.

### Changed

- Report parser now accepts an all-status multiplex stream (only `status:ok`/
  `status:error` markers, no content-bearing section). Previously such a stream
  was rejected as `unrecognized input format`; it needed at least one
  testjson/sarif body. Production `make check` output always leads with a body,
  so this only affected synthetic all-status streams.
