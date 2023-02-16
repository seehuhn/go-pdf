package tounicode

import (
	"bytes"
	"testing"
	"unicode/utf16"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/sfnt/type1"
)

func TestWrite(t *testing.T) {
	info := &Info{
		CodeSpace: []CodeSpaceRange{
			{First: 0, Last: 0xff},
		},
		Singles: []Single{
			{Code: 32, UTF16: utf16.Encode([]rune("lot's of space"))},
			{Code: 33, UTF16: nil},
		},
		Ranges: []Range{
			{
				First: 65,
				Last:  90,
				UTF16: [][]uint16{utf16.Encode([]rune("A"))},
			},
			{
				First: 100,
				Last:  102,
				UTF16: [][]uint16{utf16.Encode([]rune("fi")), utf16.Encode([]rune("fl")), utf16.Encode([]rune("ffl"))},
			},
		},
		Name: "Jochen-Chaotic-UCS2",
		ROS: &type1.CIDSystemInfo{
			Registry:   "Jochen",
			Ordering:   "Chaotic",
			Supplement: 12,
		},
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
