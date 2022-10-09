// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package sfnt_test

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/glyph"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/type1"
)

func TestGetFontInfo(t *testing.T) {
	font := debug.MakeSimpleFont()
	font.Trademark = "test trademark notice"
	font.Copyright = "(c) 2022 test copyright notice"

	fontInfo1 := font.GetFontInfo()

	cffFont1 := &cff.Font{
		FontInfo: fontInfo1,
		Outlines: font.Outlines.(*cff.Outlines),
	}
	buf := &bytes.Buffer{}
	err := cffFont1.Encode(buf)
	if err != nil {
		t.Fatal(err)
	}
	cffData := buf.Bytes()

	cffFont2, err := cff.Read(bytes.NewReader(cffData))
	if err != nil {
		t.Fatal(err)
	}
	fontInfo2 := cffFont2.FontInfo

	if d := cmp.Diff(fontInfo1, fontInfo2); d != "" {
		t.Errorf("font info differs: (-got +want)\n%s", d)
	}
}

func FuzzFont(f *testing.F) {
	f.Add(goregular.TTF)
	f.Add(gobolditalic.TTF)

	fontInfo := debug.MakeSimpleFont()
	buf := &bytes.Buffer{}
	_, err := fontInfo.Write(buf)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())

	g0 := cff.NewGlyph(".notdef", 777)
	g0.MoveTo(0, 0)
	g0.LineTo(600, 0)
	g0.LineTo(600, 600)
	g0.LineTo(0, 600)
	g1 := cff.NewGlyph("A", 900)
	g1.MoveTo(50, 50)
	g1.LineTo(850, 50)
	g1.LineTo(850, 850)
	g1.LineTo(50, 850)
	gg := []*cff.Glyph{g0, g1}
	fontInfo = &sfnt.Info{
		FamilyName:         "Test",
		Width:              font.WidthNormal,
		Weight:             font.WeightNormal,
		UnitsPerEm:         1234,
		Ascent:             800,
		Descent:            -200,
		LineGap:            100,
		CapHeight:          400,
		XHeight:            200,
		ItalicAngle:        -12.5,
		UnderlinePosition:  -100,
		UnderlineThickness: -50,
		Outlines: &cff.Outlines{
			Glyphs: gg,
			Private: []*type1.PrivateDict{
				{
					BlueValues: []funit.Int16{-10, 0, 700, 800},
					StdHW:      70,
					StdVW:      70,
				},
			},
			FdSelect: func(glyph.ID) int { return 0 },
			Encoding: cff.StandardEncoding(gg),
		},
	}
	buf.Reset()
	_, err = fontInfo.Write(buf)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())

	// fd, err := os.Open("../../demo/try-all-fonts/all-fonts")
	// if err != nil {
	// 	f.Fatal(err)
	// }
	// var fontFiles []string
	// scanner := bufio.NewScanner(fd)
	// for scanner.Scan() {
	// 	fontFiles = append(fontFiles, scanner.Text())
	// }
	// err = fd.Close()
	// if err != nil {
	// 	f.Fatal(err)
	// }
	// for _, fname := range fontFiles {
	// 	body, err := os.ReadFile(fname)
	// 	if err != nil {
	// 		f.Fatal(err)
	// 	}
	// 	if len(body) > 65536 {
	// 		fmt.Print(".")
	// 		continue
	// 	}
	// 	fmt.Print("*")
	// 	f.Add(body)
	// }
	// fmt.Println()

	f.Fuzz(func(t *testing.T, data []byte) {
		font1, err := sfnt.Read(bytes.NewReader(data))
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		_, err = font1.Write(buf)
		if err != nil {
			t.Fatal(err)
		}

		font2, err := sfnt.Read(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatal(err)
		}

		cmpFdSelectFn := cmp.Comparer(func(fn1, fn2 cff.FdSelectFn) bool {
			for gid := 0; gid < font1.NumGlyphs(); gid++ {
				if fn1(glyph.ID(gid)) != fn2(glyph.ID(gid)) {
					return false
				}
			}
			return true
		})
		cmpFloat := cmp.Comparer(func(x1, x2 float64) bool {
			d := math.Max(math.Abs(x1), math.Abs(x2)) * 1e-8
			return math.Abs(x2-x1) <= d
		})
		if diff := cmp.Diff(font1, font2, cmpFdSelectFn, cmpFloat); diff != "" {
			t.Errorf("different (-old +new):\n%s", diff)
		}
	})
}

func DisabledTestAll(t *testing.T) {
	fd, err := os.Open("../../demo/try-all-fonts/all-fonts")
	if err != nil {
		t.Fatal(err)
	}
	var fontFiles []string
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		fontFiles = append(fontFiles, scanner.Text())
	}
	err = fd.Close()
	if err != nil {
		t.Fatal(err)
	}

	numSuccess := 0
	numFail := 0
	for _, fname := range fontFiles {
		info, err := sfnt.ReadFile(fname)
		if err != nil {
			numFail++
			continue
		}
		numSuccess++

		_, err = info.Write(io.Discard)
		if err != nil {
			t.Error(err)
		}
	}
	t.Errorf("%d read, %d failed", numSuccess, numFail)
}

func DisabledTestOne(t *testing.T) {
	fname := "/usr/local/texlive/2022/texmf-dist/fonts/opentype/public/coelacanth/Coelacanth.otf"

	info, err := sfnt.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	_, err = info.Write(io.Discard)
	if err != nil {
		t.Error(err)
	}
}
