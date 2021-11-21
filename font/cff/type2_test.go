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
	subrUsed, gsubrUsed := cff.charStringDependencies(cff.charStrings[glyphID])
	fmt.Println(subrUsed, gsubrUsed)
}

func TestRoll(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	out := []float64{1, 2, 4, 5, 6, 3, 7, 8}
	roll(in[2:6], 3)
	for i, x := range in {
		if out[i] != x {
			t.Error(in, out)
			break
		}
	}
}
