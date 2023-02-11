package tounicode

import (
	"os"
	"testing"
)

func TestWrite(t *testing.T) {
	info := &Info{
		CodeSpace: []CodeSpaceRange{
			{First: 0, Last: 0xffff},
		},
		Singles: []Single{
			{
				Code: 32,
				Text: "lot's of space",
			},
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
				Text:  []string{"d'", "e'", "ffl"},
			},
		},
		Name:       "",
		Registry:   []byte("Jochen"),
		Ordering:   []byte("Chaotic"),
		Supplement: 12,
	}
	err := info.Write(os.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	t.Error("fish")
}
