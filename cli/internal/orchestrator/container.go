package orchestrator

import (
	"bytes"
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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return stdout, fmt.Errorf("container canceled: %w", ctx.Err())
		}
		// Only individual adapters may accept a scanner's documented finding exit.
		return stdout, fmt.Errorf("container exited: %w (%s)", err, stderr.String())
	}
	return stdout, nil
}
