package envsync

import (
	"bytes"
	"strings"
	"testing"
)

func TestMergeOrdersAddsAndDisables(t *testing.T) {
	example := []byte("# Application\nB=default-b\nA=default-a\nNEW=change-me\n")
	env := []byte("# private note\nA=secret\nOLD=legacy\nB=\n")
	result, err := Merge(".env.example", ".env", example, env, true, MergeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := `# Application
B=
A=secret
NEW=change-me

# --- envsync: local entries not present in the example ---
# private note
# envsync:disabled OLD
# OLD=legacy
# envsync:end-disabled OLD
`
	if string(result.Content) != want {
		t.Fatalf("content mismatch\n--- got ---\n%s--- want ---\n%s", result.Content, want)
	}
	assertActions(t, result.Actions, map[ActionKind][]string{
		ActionPreserved: {"B", "A"}, ActionAdded: {"NEW"}, ActionDisabled: {"OLD"},
	})
	second, err := Merge(".env.example", ".env", example, result.Content, true, MergeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed || !bytes.Equal(second.Content, result.Content) {
		t.Fatal("second merge was not idempotent")
	}
}

func TestMergeKeepExtra(t *testing.T) {
	result, err := Merge("example", "env", []byte("A=1\n"), []byte("EXTRA=x\nA=secret\n"), true, MergeOptions{KeepExtra: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Content), "\nEXTRA=x\n") {
		t.Fatalf("extra was not kept active:\n%s", result.Content)
	}
	if strings.Contains(string(result.Content), "envsync:disabled EXTRA") {
		t.Fatal("extra was unexpectedly disabled")
	}
}

func TestMergeRestoresReturningKey(t *testing.T) {
	disabled := strings.Join(renderDisabled(Record{Kind: RecordAssignment, Key: "RETURNED", Lines: []string{"RETURNED=old-secret"}}), "\n")
	env := []byte(localHeader + "\n" + disabled + "\n")
	result, err := Merge("example", "env", []byte("RETURNED=new-default\n"), env, true, MergeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Content) != "RETURNED=old-secret\n" {
		t.Fatalf("got %q", result.Content)
	}
	assertActions(t, result.Actions, map[ActionKind][]string{ActionRestored: {"RETURNED"}})
}

func TestMergeCreationCopiesExampleExactly(t *testing.T) {
	example := []byte("\ufeff# defaults\r\nA=1\r\n")
	result, err := Merge("example", "env", example, nil, false, MergeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.Content, example) {
		t.Fatalf("created content changed: %q", result.Content)
	}
}

func TestMergeDoesNotReplaceEmptyValue(t *testing.T) {
	result, err := Merge("example", "env", []byte("A=default\n"), []byte("A=\n"), true, MergeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Content) != "A=\n" || result.Changed {
		t.Fatalf("empty value was not preserved: %q", result.Content)
	}
}

func assertActions(t *testing.T, actions []Action, want map[ActionKind][]string) {
	t.Helper()
	got := make(map[ActionKind][]string)
	for _, action := range actions {
		got[action.Kind] = append(got[action.Kind], action.Key)
	}
	for kind, keys := range want {
		if strings.Join(got[kind], ",") != strings.Join(keys, ",") {
			t.Fatalf("%s actions = %v, want %v (all: %+v)", kind, got[kind], keys, actions)
		}
	}
}
