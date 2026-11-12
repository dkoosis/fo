package main

// Test-only convenience over parseWatchArgsWithOpts.
// parseWatchArgs splits watch args at the `--` separator. Flags before `--`
// configure the watcher; the trailing argv is the child command.
func parseWatchArgs(args []string) ([]string, error) {
	cmd, _, err := parseWatchArgsWithOpts(args)
	return cmd, err
}
