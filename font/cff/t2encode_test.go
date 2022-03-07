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

package cff

import (
	"bytes"
	"math"
	"testing"

	"github.com/go-test/deep"
	"seehuhn.de/go/pdf/font/type1"
)

func TestRoundTrip(t *testing.T) {
	meta := &type1.FontInfo{
		FontName:           "TestFont",
		Version:            "Version",
		Notice:             "Notice",
		Copyright:          "Copyright",
		FullName:           "FullName",
		FamilyName:         "FamilyName",
		Weight:             "Weight",
		ItalicAngle:        9,
		IsFixedPitch:       true,
		UnderlinePosition:  -80,
		UnderlineThickness: 40,
		FontMatrix:         []float64{1.0 / 1024, 0, 0, 1.0 / 1024, 0, 0},
	}
	private := []*type1.PrivateDict{
		{
			BlueValues: []int32{-22, 0, 500, 520, 700, 720},
			OtherBlues: []int32{-120, -100},
			BlueScale:  0.04379,
			BlueShift:  2,
			BlueFuzz:   3,
			StdHW:      23.4,
			StdVW:      34.5,
			ForceBold:  true,
		},
	}

	in := &Font{
		Info:    meta,
		Private: private,
	}

	g := &Glyph{
		Name:  ".notdef",
		Width: 1000,
	}
	g.MoveTo(50, 50)
	g.LineTo(950, 50)
	g.LineTo(950, 950)
	g.LineTo(50, 950)
	g.MoveTo(100, 900)
	g.LineTo(900, 900)
	g.LineTo(900, 100)
	g.LineTo(100, 100)
	in.Glyphs = append(in.Glyphs, g)

	g = &Glyph{
		Name:  "A",
		Width: 900,
	}
	g.MoveTo(50, 50)
	g.LineTo(850, 50)
	g.LineTo(850, 850)
	g.LineTo(50, 850)
	in.Glyphs = append(in.Glyphs, g)

	in.Encoding = standardEncoding(in.Glyphs)

	// ----------------------------------------------------------------------

	buf := &bytes.Buffer{}
	err := in.Encode(buf)
	if err != nil {
		t.Fatal(err)
	}

	// ----------------------------------------------------------------------

	out, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	for _, difference := range deep.Equal(in, out) {
		t.Error(difference)
	}

	// if !reflect.DeepEqual(in, out) {
	// 	t.Errorf("Info: %v != %v", in.Info, out.Info)
	// }
}

func TestFindEdges(t *testing.T) {
	meta := &type1.FontInfo{
		FontName:   "Test",
		FontMatrix: defaultFontMatrix,
	}
	in := &Font{
		Info:    meta,
		Private: []*type1.PrivateDict{{}},
	}

	g := NewGlyph(".notdef", 0)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.1", 100) // empty, non-zero width
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.1b", 300) // t2hmoveto
	g.MoveTo(10, 0)
	g.LineTo(20, 10)
	g.LineTo(20, 20)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.1c", 300) // t2vmoveto
	g.MoveTo(0, 10)
	g.LineTo(20, 10)
	g.LineTo(20, 20)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.2", 200) // t2rlineto
	g.MoveTo(10, 10)
	g.LineTo(20, 20)
	g.LineTo(30, 10)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.3a", 300) // t2hlineto
	g.MoveTo(10, 10)
	g.LineTo(20, 10)
	g.LineTo(20, 20)
	g.LineTo(40, 20)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.3b", 300) // t2vlineto
	g.MoveTo(10, 10)
	g.LineTo(10, 20)
	g.LineTo(20, 20)
	g.LineTo(20, 30)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.4a", 400) // t2rrcurveto
	g.MoveTo(10, 10)
	g.CurveTo(20, 100, 90, 90, 100, 100)
	g.CurveTo(10, 20, 30, 40, 50, 60)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.4b", 400) // t2rlinecurve
	g.MoveTo(0, 0)
	g.LineTo(1, 2)
	g.CurveTo(3, 4, 5, 6, 7, 8)
	g.CurveTo(9, 10, 11, 12, 13, 14)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.4c", 400) // t2rcurveline
	g.MoveTo(1, 2)
	g.CurveTo(3, 4, 5, 6, 7, 8)
	g.CurveTo(9, 10, 11, 12, 13, 14)
	g.LineTo(15, 16)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.5a", 500) // t2hhcurveto
	g.MoveTo(1, 2)
	g.CurveTo(10, 2, 11, 12, 13, 12)
	g.CurveTo(14, 12, 15, 16, 17, 16)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.5b", 500) // t2hhcurveto, first segment not horizontal
	g.MoveTo(1, 1)
	g.CurveTo(10, 2, 11, 12, 13, 12)
	g.CurveTo(14, 12, 15, 16, 17, 16)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.5c", 500) // t2vvcurveto
	g.MoveTo(1, 2)
	g.CurveTo(1, 4, 7, 6, 7, 8)
	g.CurveTo(7, 10, 13, 12, 13, 14)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.5d", 500) // t2vvcurveto, first segment not vertical
	g.MoveTo(1, 2)
	g.CurveTo(3, 4, 7, 6, 7, 8)
	g.CurveTo(7, 10, 13, 12, 13, 14)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.6a", 600) // t2hvcurveto
	g.MoveTo(1, 2)
	g.CurveTo(3, 2, 7, 6, 7, 8)
	g.CurveTo(7, 10, 11, 14, 13, 14)
	g.CurveTo(15, 14, 19, 18, 19, 20)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.6b", 600) // t2hvcurveto, last segment not vertical
	g.MoveTo(1, 2)
	g.CurveTo(3, 2, 5, 6, 7, 8)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.6c", 600) // t2vhcurveto
	g.MoveTo(1, 2)
	g.CurveTo(1, 4, 5, 8, 7, 8)
	g.CurveTo(9, 8, 13, 12, 13, 14)
	g.CurveTo(13, 16, 17, 20, 19, 20)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.6d", 600) // t2vhcurveto, last segment not horizontal
	g.MoveTo(1, 2)
	g.CurveTo(1, 4, 5, 6, 7, 8)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.7a", 700) // t2hflex
	g.MoveTo(0, 0)
	g.CurveTo(1, 0, 2, 2.5, 3, 2.5)
	g.CurveTo(4, 2.5, 5, 0, 6, 0)
	in.Glyphs = append(in.Glyphs, g)

	g = NewGlyph("test.7b", 700) // t2hflex1
	g.MoveTo(0, 0)
	g.CurveTo(1, 1, 2, 2.5, 3, 2.5)
	g.CurveTo(4, 2.5, 5, 1, 6, 0)
	in.Glyphs = append(in.Glyphs, g)

	in.Encoding = standardEncoding(in.Glyphs)

	buf := &bytes.Buffer{}
	err := in.Encode(buf)
	if err != nil {
		t.Fatal(err)
	}

	// ----------------------------------------------------------------------

	out, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range deep.Equal(in, out) {
		t.Error(err)
	}
	// if !reflect.DeepEqual(in, out) {
	// 	fmt.Println(in)
	// 	fmt.Println(out)
	// 	t.Error("different")
	// }
}

func TestType2EncodeNumber(t *testing.T) {
	cases := []float64{
		0, 1, -1, 10, -10, 100, -100, 1000, -1000, 10000, -10000,
		0.5, -0.5, 1.5, -1.5, 2.5, -2.5, 3.5, -3.5, 4.5, -4.5, 5.5, -5.5,
		1 / 65536., -1 / 65536., 10 / 65536., -10 / 65536., 100 / 65536., -100 / 65536.,
		12345.67,
	}

	// The decoder for Type 2 number is buried inside the CFF parser.  For
	// testing, we encode integers into arguments of a moveto command in a
	// charstring, and then use the decoder to this charstring.
	info := &decodeInfo{}
	for _, test := range cases {
		enc := encodeNumber(f16(test))

		if math.Abs(enc.Val.Float64()-test) > 0.5/65536 {
			t.Errorf("%f != %f", enc.Val.Float64(), test)
			continue
		}

		var code []byte
		code = append(code, enc.Code...)
		code = append(code, t2hmoveto.Bytes()...)
		code = append(code, t2endchar.Bytes()...)

		glyph, err := decodeCharString(info, code)
		if err != nil {
			t.Fatal(err)
		}
		args := glyph.Cmds[0].Args
		if math.Abs(args[0]-enc.Val.Float64()) > 1e-10 {
			t.Errorf("%f != %f", args[0], enc.Val.Float64())
		}
	}
}

func TestType2EncodeInt(t *testing.T) {
	// The decoder for Type 2 integers is buried inside the CFF parser.  For
	// testing, we encode integers into arguments of a moveto command in a
	// charstring, and then use the decoder to this charstring.
	info := &decodeInfo{}
	for i := -2000; i <= 2000; i += 2 {
		var code []byte
		code = append(code, encodeInt(int16(i))...)
		code = append(code, encodeInt(int16(i+1))...)
		code = append(code, t2rmoveto.Bytes()...)
		code = append(code, t2endchar.Bytes()...)

		glyph, err := decodeCharString(info, code)
		if err != nil {
			t.Fatal(err)
		}
		args := glyph.Cmds[0].Args
		if args[0] != float64(i) || args[1] != float64(i+1) {
			t.Fatalf("%f,%f != %d,%d", args[0], args[1], i, i+1)
		}
	}
}
