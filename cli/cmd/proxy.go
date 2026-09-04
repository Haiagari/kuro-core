package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kuro/git-proxy/server"
)

// RunProxy handles `kuro proxy [--addr] [--upstream]`.
// It starts the fail-closed local Git Smart-HTTP proxy in-process.
func RunProxy(args []string) {
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", "", "listen address (default :8000 or $LISTEN_ADDR)")
	upstream := fs.String("upstream", "", "upstream git forge URL (default https://github.com or $UPSTREAM_URL)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if *addr != "" {
		_ = os.Setenv("LISTEN_ADDR", *addr)
	}
	if *upstream != "" {
		_ = os.Setenv("UPSTREAM_URL", *upstream)
	}

	listen := envOr("LISTEN_ADDR", ":8000")
	up := envOr("UPSTREAM_URL", "https://github.com")
	scanMode := envOr("SCAN_MODE", "local")

	fmt.Printf("Kuro Core Git Proxy listening on %s → %s (SCAN_MODE=%s)\n", listen, up, scanMode)
	fmt.Println("Tip: git remote add proxy http://localhost:8000/<owner>/<repo>.git && git push proxy <branch>")
	if scanMode == "local" {
		fmt.Println("     Ensure kuro is on PATH or set KURO_BIN (e.g. KURO_BIN=./bin/kuro)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "proxy error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
