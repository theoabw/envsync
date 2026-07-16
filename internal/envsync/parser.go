package envsync

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var assignmentStart = regexp.MustCompile(`^[ \t]*(export[ \t]+)?([A-Za-z_][A-Za-z0-9_.-]*)[ \t]*=`)

const (
	disabledBegin = "# envsync:disabled "
	disabledEnd   = "# envsync:end-disabled "
)

func Parse(path string, data []byte) (Document, error) {
	doc := Document{Newline: "\n"}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		doc.BOM = true
		data = data[3:]
	}
	if bytes.Contains(data, []byte("\r\n")) {
		doc.Newline = "\r\n"
	}
	doc.FinalNewline = bytes.HasSuffix(data, []byte("\n")) || bytes.HasSuffix(data, []byte("\r"))
	text := normalizeNewline(string(data))
	if doc.FinalNewline && strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text, "\n")
	}
	if text == "" {
		return doc, nil
	}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); {
		lineNo := i + 1
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			doc.Records = append(doc.Records, Record{Kind: RecordBlank, Lines: []string{line}, Raw: line})
			i++
			continue
		}
		if strings.HasPrefix(trimmed, disabledBegin) {
			key := strings.TrimSpace(strings.TrimPrefix(trimmed, disabledBegin))
			if !validKey(key) {
				return Document{}, &ParseError{Path: path, Line: lineNo, Msg: "invalid envsync disabled marker"}
			}
			var original []string
			j := i + 1
			for ; j < len(lines); j++ {
				current := lines[j]
				if strings.TrimSpace(current) == disabledEnd+key {
					break
				}
				if !strings.HasPrefix(current, "# ") {
					return Document{}, &ParseError{Path: path, Line: j + 1, Msg: "disabled block content must begin with '# '"}
				}
				original = append(original, strings.TrimPrefix(current, "# "))
			}
			if j == len(lines) {
				return Document{}, &ParseError{Path: path, Line: lineNo, Msg: "unterminated envsync disabled block"}
			}
			if len(original) == 0 {
				return Document{}, &ParseError{Path: path, Line: lineNo, Msg: "empty envsync disabled block"}
			}
			parsedKey, consumed, err := assignmentRecord(original, 0)
			if err != nil || parsedKey != key || consumed != len(original) {
				return Document{}, &ParseError{Path: path, Line: lineNo, Msg: "disabled block does not contain its declared assignment"}
			}
			rawLines := append([]string{line}, lines[i+1:j+1]...)
			doc.Records = append(doc.Records, Record{Kind: RecordDisabled, Key: key, Lines: original, Raw: strings.Join(rawLines, "\n")})
			i = j + 1
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			doc.Records = append(doc.Records, Record{Kind: RecordComment, Lines: []string{line}, Raw: line})
			i++
			continue
		}
		key, consumed, err := assignmentRecord(lines, i)
		if err != nil {
			return Document{}, &ParseError{Path: path, Line: lineNo, Msg: err.Error()}
		}
		recordLines := append([]string(nil), lines[i:i+consumed]...)
		doc.Records = append(doc.Records, Record{Kind: RecordAssignment, Key: key, Lines: recordLines, Raw: strings.Join(recordLines, "\n")})
		i += consumed
	}
	return doc, validateDuplicates(path, doc)
}

func assignmentRecord(lines []string, start int) (string, int, error) {
	line := lines[start]
	m := assignmentStart.FindStringSubmatchIndex(line)
	if m == nil {
		return "", 0, fmt.Errorf("expected KEY=value, optional export, a comment, or a blank line")
	}
	key := line[m[4]:m[5]]
	eq := strings.Index(line[m[0]:m[1]], "=") + m[0]
	rest := strings.TrimLeft(line[eq+1:], " \t")
	if rest == "" || (rest[0] != '\'' && rest[0] != '"') {
		return key, 1, nil
	}
	quote := rest[0]
	if quoteClosed(rest[1:], quote) {
		return key, 1, nil
	}
	for i := start + 1; i < len(lines); i++ {
		if quoteClosed(lines[i], quote) {
			return key, i - start + 1, nil
		}
	}
	return "", 0, fmt.Errorf("unterminated %c-quoted value", quote)
}

func quoteClosed(s string, quote byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != quote {
			continue
		}
		if quote == '\'' {
			return true
		}
		backslashes := 0
		for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return true
		}
	}
	return false
}

func validKey(key string) bool {
	return assignmentStart.MatchString(key + "=")
}

func validateDuplicates(path string, doc Document) error {
	seen := make(map[string]struct{})
	for _, r := range doc.Records {
		if r.Kind != RecordAssignment {
			continue
		}
		if _, ok := seen[r.Key]; ok {
			return fmt.Errorf("%s: duplicate active key %q", path, r.Key)
		}
		seen[r.Key] = struct{}{}
	}
	return nil
}

func renderDisabled(r Record) []string {
	lines := []string{disabledBegin + r.Key}
	for _, line := range r.Lines {
		lines = append(lines, "# "+line)
	}
	return append(lines, disabledEnd+r.Key)
}
