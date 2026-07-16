package envsync

import (
	"fmt"
	"strings"
)

type RecordKind int

const (
	RecordBlank RecordKind = iota
	RecordComment
	RecordAssignment
	RecordDisabled
)

type Record struct {
	Kind  RecordKind
	Key   string
	Lines []string
	Raw   string
}

type Document struct {
	Records      []Record
	Newline      string
	FinalNewline bool
	BOM          bool
}

type ActionKind string

const (
	ActionAdded     ActionKind = "added"
	ActionPreserved ActionKind = "preserved"
	ActionDisabled  ActionKind = "disabled"
	ActionKept      ActionKind = "kept"
	ActionRestored  ActionKind = "restored"
)

type Action struct {
	Kind ActionKind
	Key  string
}

type MergeOptions struct {
	KeepExtra bool
}

type MergeResult struct {
	Content []byte
	Actions []Action
	Changed bool
}

type ParseError struct {
	Path string
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Msg)
}

func normalizeNewline(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
