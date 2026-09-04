package orchestrator

import (
	"context"
	"os/exec"
	"time"
)

// ── Runtime detection ──────────────────────────────────────

func detectRuntime() string {
	// First, check if docker binary exists AND daemon responds
	if _, err := exec.LookPath("docker"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "docker", "info").Run(); err == nil {
			return "docker"
		}
	}
	// Second, check if podman binary exists AND daemon/CLI works
	if _, err := exec.LookPath("podman"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "podman", "info").Run(); err == nil {
			return "podman"
		}
		// Fallback: podman binary exists even if info timed out
		return "podman"
	}
	// Fallback to docker
	return "docker"
}
