package cff

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/go-test/deep"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

func FuzzEncoding(f *testing.F) {
	ss := &cffStrings{}
	var cc []int32

	var glyphs []*Glyph
	for i := 0; i < 258; i++ {
		var name string
		if i == 0 {
			name = ".notdef"
		} else if i >= 'A' && i <= 'Z' {
			name = string([]rune{rune(i)})
		} else {
			name = fmt.Sprintf("%d", i)
		}
		glyphs = append(glyphs, &Glyph{Name: pdf.Name(name)})
		cc = append(cc, ss.lookup(name))
	}

	f.Fuzz(func(t *testing.T, data1 []byte) {
		p := parser.New(bytes.NewReader(data1))
		err := p.SetRegion("test", 0, int64(len(data1)))
		if err != nil {
			t.Fatal(err)
		}
		enc1, err := readEncoding(p, cc)
		if err != nil {
			return
		}

		var enc2 []font.GlyphID
		var data2 []byte
		if isStandardEncoding(enc1, glyphs) {
			enc2 = standardEncoding(glyphs)
		} else if isExpertEncoding(enc1, glyphs) {
			enc2 = expertEncoding(glyphs)
		} else {
			data2, err = encodeEncoding(enc1, cc)
			if err != nil {
				t.Fatal(err)
			}

			p = parser.New(bytes.NewReader(data2))
			err = p.SetRegion("test", 0, int64(len(data2)))
			if err != nil {
				t.Fatal(err)
			}
			enc2, err = readEncoding(p, cc)
			if err != nil {
				t.Fatal(err)
			}
		}

		for _, err := range deep.Equal(enc1, enc2) {
			t.Error(err)
		}
	})
}
