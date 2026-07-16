package envsync

import (
	"bytes"
	"fmt"
	"strings"
)

const localHeader = "# --- envsync: local entries not present in the example ---"

func Merge(examplePath, envPath string, exampleData, envData []byte, envExists bool, opts MergeOptions) (MergeResult, error) {
	example, err := Parse(examplePath, exampleData)
	if err != nil {
		return MergeResult{}, err
	}
	if !envExists {
		var actions []Action
		for _, r := range example.Records {
			if r.Kind == RecordAssignment {
				actions = append(actions, Action{Kind: ActionAdded, Key: r.Key})
			}
		}
		return MergeResult{Content: append([]byte(nil), exampleData...), Actions: actions, Changed: true}, nil
	}
	env, err := Parse(envPath, envData)
	if err != nil {
		return MergeResult{}, err
	}
	active := make(map[string]Record)
	disabled := make(map[string]Record)
	for _, r := range env.Records {
		switch r.Kind {
		case RecordAssignment:
			active[r.Key] = r
		case RecordDisabled:
			if _, exists := disabled[r.Key]; exists {
				return MergeResult{}, fmt.Errorf("%s: duplicate disabled key %q", envPath, r.Key)
			}
			disabled[r.Key] = r
		}
	}
	for key := range active {
		if _, exists := disabled[key]; exists {
			return MergeResult{}, fmt.Errorf("%s: key %q is both active and managed-disabled", envPath, key)
		}
	}
	exampleKeys := make(map[string]struct{})
	var output []string
	var actions []Action
	for _, r := range example.Records {
		if r.Kind != RecordAssignment {
			output = append(output, r.Lines...)
			continue
		}
		exampleKeys[r.Key] = struct{}{}
		if current, ok := active[r.Key]; ok {
			output = append(output, current.Lines...)
			actions = append(actions, Action{Kind: ActionPreserved, Key: r.Key})
			continue
		}
		if old, ok := disabled[r.Key]; ok {
			output = append(output, old.Lines...)
			actions = append(actions, Action{Kind: ActionRestored, Key: r.Key})
			continue
		}
		output = append(output, r.Lines...)
		actions = append(actions, Action{Kind: ActionAdded, Key: r.Key})
	}

	exampleComments := commentCounts(example)
	var local []string
	for _, r := range env.Records {
		switch r.Kind {
		case RecordComment:
			if strings.TrimSpace(r.Raw) == localHeader {
				continue
			}
			if exampleComments[r.Raw] > 0 {
				exampleComments[r.Raw]--
				continue
			}
			local = append(local, r.Lines...)
		case RecordAssignment:
			if _, known := exampleKeys[r.Key]; known {
				continue
			}
			if opts.KeepExtra {
				local = append(local, r.Lines...)
				actions = append(actions, Action{Kind: ActionKept, Key: r.Key})
			} else {
				local = append(local, renderDisabled(r)...)
				actions = append(actions, Action{Kind: ActionDisabled, Key: r.Key})
			}
		case RecordDisabled:
			if _, known := exampleKeys[r.Key]; known {
				continue
			}
			local = append(local, renderDisabled(r)...)
		}
	}
	local = trimBlankLines(local)
	output = trimBlankLines(output)
	if len(local) > 0 {
		if len(output) > 0 {
			output = append(output, "")
		}
		output = append(output, localHeader)
		output = append(output, local...)
	}
	newline := env.Newline
	finalNewline := env.FinalNewline
	if len(envData) == 0 {
		newline = example.Newline
		finalNewline = example.FinalNewline
	}
	content := strings.Join(output, newline)
	if finalNewline && len(output) > 0 {
		content += newline
	}
	if env.BOM {
		content = "\ufeff" + content
	}
	result := []byte(content)
	return MergeResult{Content: result, Actions: actions, Changed: !bytes.Equal(result, envData)}, nil
}

func commentCounts(doc Document) map[string]int {
	counts := make(map[string]int)
	for _, r := range doc.Records {
		if r.Kind == RecordComment {
			counts[r.Raw]++
		}
	}
	return counts
}

func trimBlankLines(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}
