package cff

import (
	"bytes"
	"os"
	"reflect"
	"testing"

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
		ItalicAngle:        1.23,
		IsFixedPitch:       true,
		UnderlinePosition:  -80,
		UnderlineThickness: 40,
		PaintType:          0,
		FontMatrix:         []float64{1.0 / 1024, 0, 0, 1.0 / 1024, 0, 0},
		Private: []*type1.PrivateDict{
			{
				BlueValues: []int32{-22, 0, 500, 520, 700, 720},
				OtherBlues: []int32{-120, -100},
				BlueScale:  1,
				BlueShift:  2,
				BlueFuzz:   3,
				StdHW:      23.4,
				StdVW:      34.5,
				ForceBold:  true,
			},
		},
	}

	in := &Font{
		Info: meta,
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

	if len(out.Glyphs) != len(in.Glyphs) {
		t.Fatalf("wrong number of glyphs: %d != %d", len(out.Glyphs), len(in.Glyphs))
	}

	target := []int32{1000, 900}
	for i, g := range out.Glyphs {
		if g.Width != target[i] {
			t.Fatalf("wrong glyph %d width: %d != %d", i, g.Width, target[i])
		}
	}

	if !reflect.DeepEqual(in.Info, out.Info) {
		t.Errorf("Info: %v != %v", in.Info, out.Info)
	}
}

func TestCreate(t *testing.T) {
	meta := &type1.FontInfo{
		FontName: "Test",
		Private:  []*type1.PrivateDict{{}},
	}
	cff := &Font{
		Info: meta,
	}

	g := &Glyph{
		Name:  ".notdef",
		Width: 500,
	}
	g.MoveTo(50, 50)
	g.LineTo(450, 50)
	g.LineTo(450, 950)
	g.LineTo(50, 950)
	g.MoveTo(100, 900)
	g.LineTo(400, 900)
	g.LineTo(400, 100)
	g.LineTo(100, 100)
	cff.Glyphs = append(cff.Glyphs, g)

	// an arrow pointing right
	g = &Glyph{
		Name:  "A",
		Width: 1000,
	}
	g.MoveTo(0, 450)
	g.LineTo(700, 450)
	g.LineTo(700, 350)
	g.LineTo(1000, 500)
	g.LineTo(700, 650)
	g.LineTo(700, 550)
	g.LineTo(0, 550)
	cff.Glyphs = append(cff.Glyphs, g)

	fd, err := os.Create("test.cff")
	if err != nil {
		t.Fatal(err)
	}
	err = cff.Encode(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncodeIntType2(t *testing.T) {
	// The decode for Type 2 integers is buried in the CFF parser.  For
	// testing, we encode integers into arguments of a moveto command in a
	// charstring, and then decode this charstring.
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
