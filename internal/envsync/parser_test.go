package envsync

import (
	"strings"
	"testing"
)

func TestParseCommonSyntaxAndMultiline(t *testing.T) {
	input := "\ufeff# heading\r\nexport A=one # comment\r\nB=\"line 1\r\nline 2\\\" still quoted\r\nline 3\"\r\nEMPTY=\r\nDOTTED.KEY-name='x#y=z'\r\n"
	doc, err := Parse("test.env", []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.BOM || doc.Newline != "\r\n" || !doc.FinalNewline {
		t.Fatalf("unexpected document metadata: %+v", doc)
	}
	var got []string
	for _, record := range doc.Records {
		if record.Kind == RecordAssignment {
			got = append(got, record.Key)
		}
	}
	want := []string{"A", "B", "EMPTY", "DOTTED.KEY-name"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	if len(doc.Records[2].Lines) != 3 {
		t.Fatalf("multiline record has %d lines", len(doc.Records[2].Lines))
	}
}

func TestParseRejectsMalformedAndDuplicates(t *testing.T) {
	tests := map[string]string{
		"unknown line":       "source other.env\n",
		"unterminated quote": "A=\"oops\n",
		"duplicate":          "A=1\nA=2\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(name, []byte(input)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDisabledRecordRoundTrip(t *testing.T) {
	original := Record{Kind: RecordAssignment, Key: "CERT", Lines: []string{"CERT=\"first", "second\""}}
	data := strings.Join(renderDisabled(original), "\n") + "\n"
	doc, err := Parse(".env", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 1 || doc.Records[0].Kind != RecordDisabled {
		t.Fatalf("unexpected records: %+v", doc.Records)
	}
	if strings.Join(doc.Records[0].Lines, "\n") != strings.Join(original.Lines, "\n") {
		t.Fatalf("restored lines differ: %#v", doc.Records[0].Lines)
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	f.Add([]byte("A=1\n"))
	f.Add([]byte("A=\"one\ntwo\"\n"))
	f.Add([]byte("# envsync:disabled A\n# A=1\n# envsync:end-disabled A\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse("fuzz.env", data)
	})
}
