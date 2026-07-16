package envsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDefaultNamedAndRecursivePairs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".env.example"), "A=1\n")
	writeTestFile(t, filepath.Join(root, ".env.production.example"), "A=1\n")
	writeTestFile(t, filepath.Join(root, "service", ".env.local.example"), "A=1\n")
	writeTestFile(t, filepath.Join(root, "node_modules", "pkg", ".env.example"), "A=1\n")

	pairs, err := Discover(DiscoverOptions{Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("non-recursive pairs = %d, want 2: %+v", len(pairs), pairs)
	}
	pairs, err = Discover(DiscoverOptions{Dir: root, Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 3 {
		t.Fatalf("recursive pairs = %d, want 3: %+v", len(pairs), pairs)
	}
}

func TestDiscoverFiltersAndExactPair(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".env.example"), "A=1\n")
	writeTestFile(t, filepath.Join(root, ".env.test.example"), "A=1\n")
	pairs, err := Discover(DiscoverOptions{Dir: root, Matches: []string{".env.*.example"}, Excludes: []string{"*.test.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 0 {
		t.Fatalf("filtered pairs = %+v", pairs)
	}
	pairs, err = Discover(DiscoverOptions{Dir: root, Example: "custom.template", Env: "custom.env"})
	if err != nil {
		t.Fatal(err)
	}
	if pairs[0].Example != filepath.Join(root, "custom.template") || pairs[0].Env != filepath.Join(root, "custom.env") {
		t.Fatalf("exact pair = %+v", pairs[0])
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
