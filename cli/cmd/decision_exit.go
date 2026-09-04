package cmd

import "strings"

// decisionExitCode maps a scan decision to a process exit code.
// pass → 0, review → 2, block/unknown → 1.
func decisionExitCode(decision string) int {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "pass", "approved", "ok":
		return 0
	case "review":
		return 2
	default:
		return 1
	}
}
