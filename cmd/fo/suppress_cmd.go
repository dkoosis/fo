package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/suppress"
)

// runSuppress dispatches `fo suppress {add,list,remove}`. Reads and
// writes .fo/ignore (or $FO_IGNORE) via pkg/suppress. Writes are atomic
// (temp file + rename inside the same directory). Echoes the mutated
// rule to stdout so callers in scripts can confirm the action.
func runSuppress(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "fo suppress: subcommand required (add|list|remove)")
		return 2
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "add":
		return runSuppressAdd(rest, stdout, stderr)
	case "list":
		return runSuppressList(rest, stdout, stderr)
	case "remove", "rm":
		return runSuppressRemove(rest, stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, suppressUsage)
		return 0
	default:
		fmt.Fprintf(stderr, "fo suppress: unknown subcommand %q (want add|list|remove)\n", sub)
		return 2
	}
}

const suppressUsage = `Usage: fo suppress <add|list|remove> [args]

  fo suppress add <rule-id> [--glob=PATTERN] [--until=YYYY-MM-DD] [--reason=TEXT]
  fo suppress list
  fo suppress remove <rule-id>

Reads and writes .fo/ignore (or $FO_IGNORE). Atomic writes.`

func runSuppressAdd(args []string, stdout, stderr io.Writer) int {
	// stdlib flag.Parse stops at the first non-flag positional, so
	// `add <rule-id> --glob=...` would drop the trailing flags. Pull the
	// positional out first, then parse the rest.
	ruleID, flagArgs, err := extractPositional(args)
	if err != nil {
		fmt.Fprintf(stderr, "fo suppress add: %v\n", err)
		return 2
	}
	fset := flag.NewFlagSet("fo suppress add", flag.ContinueOnError)
	fset.SetOutput(stderr)
	glob := fset.String("glob", "", "Path glob pattern (default: ** — match everywhere)")
	until := fset.String("until", "", "Expiry date YYYY-MM-DD (default: never)")
	reason := fset.String("reason", "", "Free-text reason for the suppression")
	if perr := fset.Parse(flagArgs); perr != nil {
		if errors.Is(perr, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(fset.Args()) > 0 {
		fmt.Fprintf(stderr, "fo suppress add: unexpected extra args: %v\n", fset.Args())
		return 2
	}
	rule := suppress.Suppression{RuleID: ruleID, Glob: suppress.DefaultGlob}
	if *glob != "" {
		rule.Glob = *glob
	}
	if *until != "" {
		t, err := time.Parse("2006-01-02", *until)
		if err != nil {
			fmt.Fprintf(stderr, "fo suppress add: --until: %v\n", err)
			return 2
		}
		if t.Year() <= 1 {
			fmt.Fprintf(stderr, "fo suppress add: --until: zero-year %q\n", *until)
			return 2
		}
		rule.Until = &t
	}
	rule.Reason = *reason

	path := suppressPath()
	existing, err := loadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "fo suppress add: %v\n", err)
		return 2
	}

	// Idempotent: drop any prior entry with the same RuleID before
	// appending the new one. Keeps the file from accreting duplicates
	// when callers re-run `add` after editing flags.
	filtered := existing[:0]
	for _, s := range existing {
		if s.RuleID != rule.RuleID {
			filtered = append(filtered, s)
		}
	}
	filtered = append(filtered, rule)

	if err := writeFile(path, filtered); err != nil {
		fmt.Fprintf(stderr, "fo suppress add: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "added: %s\n", rule.Format())
	return 0
}

func runSuppressList(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "fo suppress list: takes no arguments")
		return 2
	}
	rules, err := loadFile(suppressPath())
	if err != nil {
		fmt.Fprintf(stderr, "fo suppress list: %v\n", err)
		return 2
	}
	now := time.Now()
	for _, s := range rules {
		marker := ""
		if s.Expired(now) {
			marker = "  (expired)"
		}
		fmt.Fprintf(stdout, "%s%s\n", s.Format(), marker)
	}
	return 0
}

func runSuppressRemove(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "fo suppress remove: exactly one <rule-id> required")
		return 2
	}
	target := args[0]
	path := suppressPath()
	existing, err := loadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "fo suppress remove: %v\n", err)
		return 2
	}
	kept := make([]suppress.Suppression, 0, len(existing))
	removed := 0
	for _, s := range existing {
		if s.RuleID == target {
			removed++
			continue
		}
		kept = append(kept, s)
	}
	if removed == 0 {
		fmt.Fprintf(stderr, "fo suppress remove: %q not found in %s\n", target, path)
		return 1
	}
	if err := writeFile(path, kept); err != nil {
		fmt.Fprintf(stderr, "fo suppress remove: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "removed: %s (%d)\n", target, removed)
	return 0
}

// extractPositional pulls the first non-flag token out of args and
// returns it alongside the remaining flag-shaped tokens. Errors when
// there is no positional or more than one. Treats anything starting
// with "-" as a flag.
func extractPositional(args []string) (string, []string, error) {
	var pos string
	var rest []string
	found := false
	for _, a := range args {
		if !found && !strings.HasPrefix(a, "-") {
			pos = a
			found = true
			continue
		}
		rest = append(rest, a)
	}
	if !found {
		return "", nil, errors.New("exactly one <rule-id> required")
	}
	return pos, rest, nil
}

// loadFile parses the .fo/ignore at path. Absent file → no rules,
// nil error (treated as empty).
func loadFile(path string) ([]suppress.Suppression, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return suppress.Parse(f)
}

// writeFile renders rules to path atomically (temp file + rename in the
// same directory). Creates parent directories as needed.
func writeFile(path string, rules []suppress.Suppression) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	for _, s := range rules {
		buf.WriteString(s.Format())
		buf.WriteByte('\n')
	}
	tmp, err := os.CreateTemp(dir, ".ignore.tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, &buf); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

