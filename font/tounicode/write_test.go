package tounicode

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWrite(t *testing.T) {
	info := &Info{
		CodeSpace: []CodeSpaceRange{
			{First: 0, Last: 0xff},
		},
		Singles: []Single{
			{Code: 32, Text: "lot's of space"},
			{Code: 33, Text: ""},
		},
		Ranges: []Range{
			{
				First: 65,
				Last:  90,
				Text:  []string{"A"},
			},
			{
				First: 100,
				Last:  102,
				Text:  []string{"fi", "fl", "ffl"},
			},
		},
		Name:       "Jochen-Chaotic-UCS2",
		Registry:   []byte("Jochen"),
		Ordering:   []byte("Chaotic"),
		Supplement: 12,
	}

	buf := &bytes.Buffer{}
	err := info.Write(buf)
	if err != nil {
		t.Fatal(err)
	}

	info2, err := Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(info, info2); d != "" {
		t.Fatal(d)
	}
}
