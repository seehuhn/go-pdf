package debug

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/sfntcff"
)

func TestDebugFont(t *testing.T) {
	info, err := Build()
	outlines := info.Outlines.(*sfntcff.GlyfOutlines)

	if err != nil {
		t.Fatal(err)
	}
	for c := 'A'; c <= 'Z'; c++ {
		gid := info.CMap.Lookup(c)
		gl := outlines.Glyphs[gid]
		if g, ok := gl.Data.(glyf.SimpleGlyph); ok {
			fmt.Println("glyph", c)
			glyphInfo, err := g.Decode()
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, c := range glyphInfo.Contours {
				for _, p := range c {
					fmt.Printf("%v\n", p)
				}
				fmt.Println()
			}
		} else {
			fmt.Printf("? %c %d %T\n", c, gid, gl.Data)
		}
	}
	t.Fatal("fish")
}
