//go:build !windows

package envsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeAndApplyCreatesPrivateFile(t *testing.T) {
	root := t.TempDir()
	example := filepath.Join(root, ".env.example")
	env := filepath.Join(root, ".env")
	writeTestFile(t, example, "A=default\n")
	plan, err := Analyze(Pair{Example: example, Env: env}, FileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(env)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestApplyPreservesModeAndCreatesBackup(t *testing.T) {
	root := t.TempDir()
	example := filepath.Join(root, ".env.example")
	env := filepath.Join(root, ".env")
	writeTestFile(t, example, "A=default\nB=2\n")
	if err := os.WriteFile(env, []byte("A=secret\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	plan, err := Analyze(Pair{Example: example, Env: env}, FileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	backup, err := Apply(plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("backup path was empty")
	}
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupData) != "A=secret\n" {
		t.Fatalf("backup = %q", backupData)
	}
	info, err := os.Stat(env)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

func TestApplyDetectsConcurrentCreation(t *testing.T) {
	root := t.TempDir()
	example := filepath.Join(root, ".env.example")
	env := filepath.Join(root, ".env")
	writeTestFile(t, example, "A=default\n")
	plan, err := Analyze(Pair{Example: example, Env: env}, FileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, env, "A=someone-else\n")
	if _, err := Apply(plan, false); err == nil || !strings.Contains(err.Error(), "appeared after analysis") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnalyzeRefusesAndCanFollowSymlink(t *testing.T) {
	root := t.TempDir()
	example := filepath.Join(root, ".env.example")
	target := filepath.Join(root, "secret.env")
	link := filepath.Join(root, ".env")
	writeTestFile(t, example, "A=default\nB=2\n")
	writeTestFile(t, target, "A=secret\n")
	if err := os.Symlink("secret.env", link); err != nil {
		t.Fatal(err)
	}
	pair := Pair{Example: example, Env: link}
	if _, err := Analyze(pair, FileOptions{}); err == nil {
		t.Fatal("expected symlink refusal")
	}
	plan, err := Analyze(pair, FileOptions{FollowSymlink: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, false); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink was replaced: info=%v err=%v", info, err)
	}
}
