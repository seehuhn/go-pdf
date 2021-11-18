package cff

import (
	"fmt"
	"os"
	"testing"
)

func TestCCDep(t *testing.T) {
	in, err := os.Open("SourceSerif4-Regular.cff")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	cff, err := Read(in)
	if err != nil {
		t.Fatal(err)
	}

	glyphID := 2
	fmt.Println(cff.strings.get(cff.glyphNames[glyphID]))
	cff.charStringDependencies(cff.charStrings[glyphID])
}

func TestRoll(t *testing.T) {
	in := []int32{1, 2, 3, 4, 5, 6, 7, 8}
	out := []int32{1, 2, 4, 5, 6, 3, 7, 8}
	roll(in[2:6], 3)
	for i, x := range in {
		if out[i] != x {
			t.Error(in, out)
			break
		}
	}
}
