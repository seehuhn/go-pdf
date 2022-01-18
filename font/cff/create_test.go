package cff

import (
	"bytes"
	"fmt"
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
		BlueValues:         []int32{-22, 0, 500, 520, 700, 720},
		OtherBlues:         []int32{-120, -100},
		BlueScale:          1,
		BlueShift:          2,
		BlueFuzz:           3,
		StdHW:              23.4,
		StdVW:              34.5,
		ForceBold:          true,
	}
	b := NewBuilder(meta, 1000, 1000)

	g, err := b.AddGlyph(".notdef")
	if err != nil {
		t.Fatal(err)
	}
	g.SetWidth(1000)
	g.MoveTo(50, 50)
	g.LineTo(950, 50)
	g.LineTo(950, 950)
	g.LineTo(50, 950)
	g.MoveTo(100, 900)
	g.LineTo(900, 900)
	g.LineTo(900, 100)
	g.LineTo(100, 100)
	g.Close()

	g, err = b.AddGlyph("A")
	g.SetWidth(900)
	g.MoveTo(50, 50)
	g.LineTo(850, 50)
	g.LineTo(850, 850)
	g.LineTo(50, 850)
	if err != nil {
		t.Fatal(err)
	}
	g.Close()

	in, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	err = in.Encode(buf)
	if err != nil {
		t.Fatal(err)
	}

	// ----------------------------------------------------------------------

	out, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	target := []int{1000, 900}
	if !reflect.DeepEqual(out.Width, target) {
		t.Errorf("Width: %v != %v", in.Width, target)
	}

	if !reflect.DeepEqual(in.Info, out.Info) {
		t.Errorf("Info: %v != %v", in.Info, out.Info)
	}
}

func TestCreate(t *testing.T) {
	meta := &type1.FontInfo{
		FontName: "Test",
	}
	b := NewBuilder(meta, 500, 500)

	g, err := b.AddGlyph(".notdef")
	if err != nil {
		t.Fatal(err)
	}
	g.SetWidth(500)
	g.MoveTo(50, 50)
	g.LineTo(450, 50)
	g.LineTo(450, 950)
	g.LineTo(50, 950)
	g.MoveTo(100, 900)
	g.LineTo(400, 900)
	g.LineTo(400, 100)
	g.LineTo(100, 100)
	g.Close()

	// an arrow pointing right
	g, err = b.AddGlyph("A")
	if err != nil {
		t.Fatal(err)
	}
	g.SetWidth(1000)
	g.MoveTo(0, 450)
	g.LineTo(700, 450)
	g.LineTo(700, 350)
	g.LineTo(1000, 500)
	g.LineTo(700, 650)
	g.LineTo(700, 550)
	g.LineTo(0, 550)
	g.Close()

	cff, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

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

func TestEncode(t *testing.T) {
	in, err := os.Open("Atkinson-Hyperlegible-BoldItalic-102.cff")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	cff, err := Read(in)
	if err != nil {
		t.Fatal(err)
	}

	gid := 4

	ctx := &encoder{}
	orig, err := cff.doDecode(ctx, cff.charStrings[gid])
	if err != nil {
		t.Fatal(err)
	}

	code := ctx.encode(cff.defaultWidth, cff.nominalWidth)
	fmt.Printf("=> % x\n", code)
	fmt.Printf(".. % x\n", orig)
}
