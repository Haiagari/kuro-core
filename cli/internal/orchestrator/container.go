package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// ── Container runtime ──────────────────────────────────────

func (a *LocalAdapter) runContainer(ctx context.Context, image string, args []string, volumes map[string]string) ([]byte, error) {
	cmdArgs := []string{"run", "--rm", "--network=none", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--memory=512m", "--cpus=1.0"}
	// Use the caller identity to read private source/rule directories without
	// changing their permissions. A writable ephemeral home supports scanner caches.
	if uid, gid := os.Getuid(), os.Getgid(); uid >= 0 && gid >= 0 {
		cmdArgs = append(cmdArgs, "--user", strconv.Itoa(uid)+":"+strconv.Itoa(gid))
		if a.runtime == "podman" {
			cmdArgs = append(cmdArgs, "--userns=keep-id")
		}
	}
	cmdArgs = append(cmdArgs, "--env", "HOME=/tmp", "--env", "GIT_CONFIG_COUNT=1", "--env", "GIT_CONFIG_KEY_0=safe.directory", "--env", "GIT_CONFIG_VALUE_0=/src")
	if a.ciMode && image == gitleaksImage {
		cmdArgs = append(cmdArgs, "--env", "GITLEAKS_CONFIG_TOML=[extend]\nuseDefault = true")
	}
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
