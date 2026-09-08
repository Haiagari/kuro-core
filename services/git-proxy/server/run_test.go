package server

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandHelpers(t *testing.T) {
	out, err := runCmd("sh", "-c", "printf command-output")
	if err != nil {
		t.Fatalf("runCmd: %v", err)
	}
	if out != "command-output" {
		t.Fatalf("runCmd output = %q, want command-output", out)
	}

	stdinFile := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(stdinFile, []byte("stdin-output"), 0o600); err != nil {
		t.Fatalf("write stdin fixture: %v", err)
	}
	out, err = runCmdWithStdin(stdinFile, "sh", "-c", "cat")
	if err != nil {
		t.Fatalf("runCmdWithStdin: %v", err)
	}
	if out != "stdin-output" {
		t.Fatalf("runCmdWithStdin output = %q, want stdin-output", out)
	}

	out, err = runCmdWithReader(strings.NewReader("reader-output"), "sh", "-c", "cat")
	if err != nil {
		t.Fatalf("runCmdWithReader: %v", err)
	}
	if out != "reader-output" {
		t.Fatalf("runCmdWithReader output = %q, want reader-output", out)
	}

	bytesOut, err := runCmdOutput("sh", "-c", "printf byte-output")
	if err != nil {
		t.Fatalf("runCmdOutput: %v", err)
	}
	if string(bytesOut) != "byte-output" {
		t.Fatalf("runCmdOutput output = %q, want byte-output", bytesOut)
	}
}

func TestCopyExtractedFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "shared")
	srcFile := filepath.Join(srcDir, "nested", "result.json")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("result"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	if err := copyExtractedFiles(srcDir, dstDir); err != nil {
		t.Fatalf("copyExtractedFiles: %v", err)
	}

	copied, err := os.Open(filepath.Join(dstDir, "nested", "result.json"))
	if err != nil {
		t.Fatalf("open copied file: %v", err)
	}
	defer copied.Close()
	contents, err := io.ReadAll(copied)
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(contents) != "result" {
		t.Fatalf("copied contents = %q, want result", contents)
	}
}
