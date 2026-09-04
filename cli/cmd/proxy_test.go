package cmd

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
)

func TestProxyFlagSetHelp(t *testing.T) {
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	_ = fs.String("addr", "", "listen address (default :8000 or $LISTEN_ADDR)")
	_ = fs.String("upstream", "", "upstream git forge URL (default https://github.com or $UPSTREAM_URL)")

	err := fs.Parse([]string{"-h"})
	if err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "addr") || !strings.Contains(out, "upstream") {
		t.Fatalf("help should mention flags, got: %s", out)
	}
}

func TestEnvOr(t *testing.T) {
	key := "KURO_PROXY_TEST_ENVOR"
	os.Unsetenv(key)
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Fatalf("want fallback, got %q", got)
	}
	os.Setenv(key, "from-env")
	defer os.Unsetenv(key)
	if got := envOr(key, "fallback"); got != "from-env" {
		t.Fatalf("want from-env, got %q", got)
	}
}
