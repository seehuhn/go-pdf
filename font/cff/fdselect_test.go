package cff

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

func FuzzFdSelect(f *testing.F) {
	const nGlyphs = 100
	fds := []FdSelectFn{
		func(gid font.GlyphID) int { return 0 },
		func(gid font.GlyphID) int { return int(gid) / 60 },
		func(gid font.GlyphID) int { return int(gid) / 4 },
		func(gid font.GlyphID) int { return int(gid) },
		func(gid font.GlyphID) int { return int(gid/5) % 5 },
	}
	for _, fd := range fds {
		f.Add(fd.Encode(nGlyphs))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		fmt.Printf("A % x\n", in)
		p := parser.New(bytes.NewReader(in))
		err := p.SetRegion("FDSelect", 0, int64(len(in)))
		if err != nil {
			t.Fatal(err)
		}
		fdSelect, err := readFDSelect(p, nGlyphs)
		if err != nil {
			return
		}

		in2 := fdSelect.Encode(nGlyphs)
		fmt.Printf("B % x\n", in2)
		if len(in2) > len(in) {
			t.Error("inefficient encoding")
		}

		p = parser.New(bytes.NewReader(in2))
		err = p.SetRegion("FDSelect", 0, int64(len(in2)))
		if err != nil {
			t.Fatal(err)
		}
		fdSelect2, err := readFDSelect(p, nGlyphs)
		if err != nil {
			t.Fatal(err)
		}

		for i := font.GlyphID(0); i < nGlyphs; i++ {
			if fdSelect(i) != fdSelect2(i) {
				t.Errorf("%d: %d != %d", i, fdSelect(i), fdSelect2(i))
			}
		}
	})
}
