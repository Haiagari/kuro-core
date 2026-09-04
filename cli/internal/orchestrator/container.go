package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
)

// ── Container runtime ──────────────────────────────────────

func (a *LocalAdapter) runContainer(ctx context.Context, image string, args []string, volumes map[string]string) ([]byte, error) {
	cmdArgs := []string{"run", "--rm", "--network=none", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--memory=512m", "--cpus=1.0"}
	for host, container := range volumes {
		cmdArgs = append(cmdArgs, "-v", host+":"+container)
	}
	cmdArgs = append(cmdArgs, image)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, a.runtime, cmdArgs...)
	stdout, err := cmd.Output()
	if err != nil {
		// Most scanners exit 1 when findings exist. Return stdout anyway.
		if stdout != nil && len(stdout) > 0 {
			return stdout, nil
		}
		return nil, fmt.Errorf("container exited: %w", err)
	}
	return stdout, nil
}
