package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDryRunRedactsValues(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env.example"), "A=default\nNEW=public-default\n")
	mustWrite(t, filepath.Join(root, ".env"), "A=super-secret\nOLD=also-secret\n")
	var out, errOut bytes.Buffer
	code := run([]string{"--dir", root, "--dry-run", "--color", "never"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	for _, secret := range []string{"super-secret", "also-secret", "public-default"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("output leaked value %q: %s", secret, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".env.bak")); !os.IsNotExist(err) {
		t.Fatal("dry-run unexpectedly wrote a file")
	}
	content, _ := os.ReadFile(filepath.Join(root, ".env"))
	if string(content) != "A=super-secret\nOLD=also-secret\n" {
		t.Fatal("dry-run modified destination")
	}
}

func TestRunCheckExitCodes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env.example"), "A=default\n")
	mustWrite(t, filepath.Join(root, ".env"), "A=secret\n")
	var out, errOut bytes.Buffer
	if code := run([]string{"--dir", root, "--check", "--color", "never"}, &out, &errOut); code != 0 {
		t.Fatalf("clean check code=%d stderr=%s", code, errOut.String())
	}
	mustWrite(t, filepath.Join(root, ".env.example"), "A=default\nB=2\n")
	out.Reset()
	errOut.Reset()
	if code := run([]string{"--dir", root, "--check", "--color", "never"}, &out, &errOut); code != 1 {
		t.Fatalf("drift check code=%d stderr=%s", code, errOut.String())
	}
}

func TestRunPreflightsAllPairsBeforeWriting(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env.example"), "A=default\nB=2\n")
	mustWrite(t, filepath.Join(root, ".env"), "A=secret\n")
	mustWrite(t, filepath.Join(root, ".env.bad.example"), "BROKEN=\"unterminated\n")
	var out, errOut bytes.Buffer
	if code := run([]string{"--dir", root, "--color", "never"}, &out, &errOut); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	content, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "A=secret\n" {
		t.Fatalf("first pair was written before preflight finished: %q", content)
	}
}

func TestHelpIsComprehensive(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--help"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d", code)
	}
	for _, phrase := range []string{"Discovery:", "Merge policy:", "Exit codes:", "--dry-run", "--keep-extra"} {
		if !strings.Contains(errOut.String(), phrase) {
			t.Fatalf("help missing %q", phrase)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
